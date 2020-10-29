// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package logdb

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/block"
	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/tx"
	sqlite3 "github.com/mattn/go-sqlite3"
)

type LogDB struct {
	path          string
	db            *sql.DB
	driverVersion string
}

var (
	GlobalLogDBInstance *LogDB
)

func setGlobalLogDBInstance(db *LogDB) {
	GlobalLogDBInstance = db
}

func GetGlobalLogDBInstance() *LogDB {
	return GlobalLogDBInstance
}

// New create or open log db at given path.
func New(path string) (logDB *LogDB, err error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if logDB == nil {
			err := db.Close()
			if err != nil {
				fmt.Println("could not close logdb error:", err)
			}
		}
	}()
	if _, err := db.Exec(eventTableSchema + transferTableSchema); err != nil {
		return nil, err
	}

	driverVer, _, _ := sqlite3.Version()
	logdbInstance := &LogDB{
		path,
		db,
		driverVer,
	}
	setGlobalLogDBInstance(logdbInstance)
	return logdbInstance, nil
}

// NewMem create a log db in ram.
func NewMem() (*LogDB, error) {
	return New(":memory:")
}

// Close close the log db.
func (db *LogDB) Close() {
	err := db.db.Close()
	if err != nil {
		fmt.Println("could not close logdb error:", err)
	}

}

func (db *LogDB) Path() string {
	return db.path
}

func (db *LogDB) Prepare(header *block.Header) *BlockBatch {
	return &BlockBatch{
		db:     db.db,
		header: header,
	}
}

func (db *LogDB) FilterEvents(ctx context.Context, filter *EventFilter) ([]*Event, error) {
	if filter == nil {
		return db.queryEvents(ctx, "SELECT * FROM event")
	}
	var args []interface{}
	stmt := "SELECT * FROM event WHERE 1"
	condition := "blockNumber"
	if filter.Range != nil {
		if filter.Range.Unit == Time {
			condition = "blockTime"
		}
		args = append(args, filter.Range.From)
		stmt += " AND " + condition + " >= ? "
		if filter.Range.To >= filter.Range.From {
			args = append(args, filter.Range.To)
			stmt += " AND " + condition + " <= ? "
		}
	}
	for i, criteria := range filter.CriteriaSet {
		if i == 0 {
			stmt += " AND ( 1"
		} else {
			stmt += " OR ( 1"
		}
		if criteria.Address != nil {
			args = append(args, criteria.Address.Bytes())
			stmt += " AND address = ? "
		}
		for j, topic := range criteria.Topics {
			if topic != nil {
				args = append(args, topic.Bytes())
				stmt += fmt.Sprintf(" AND topic%v = ?", j)
			}
		}
		stmt += ")"
	}

	if filter.Order == DESC {
		stmt += " ORDER BY blockNumber DESC,eventIndex DESC "
	} else {
		stmt += " ORDER BY blockNumber ASC,eventIndex ASC "
	}

	if filter.Options != nil {
		stmt += " limit ?, ? "
		args = append(args, filter.Options.Offset, filter.Options.Limit)
	}
	return db.queryEvents(ctx, stmt, args...)
}

func (db *LogDB) FilterTransfers(ctx context.Context, filter *TransferFilter) ([]*Transfer, error) {
	if filter == nil {
		return db.queryTransfers(ctx, "SELECT * FROM transfer")
	}
	var args []interface{}
	stmt := "SELECT * FROM transfer WHERE 1"
	condition := "blockNumber"
	if filter.Range != nil {
		if filter.Range.Unit == Time {
			condition = "blockTime"
		}
		args = append(args, filter.Range.From)
		stmt += " AND " + condition + " >= ? "
		if filter.Range.To >= filter.Range.From {
			args = append(args, filter.Range.To)
			stmt += " AND " + condition + " <= ? "
		}
	}
	if filter.TxID != nil {
		args = append(args, filter.TxID.Bytes())
		stmt += " AND txID = ? "
	}
	length := len(filter.CriteriaSet)
	if length > 0 {
		for i, criteria := range filter.CriteriaSet {
			if i == 0 {
				stmt += " AND (( 1 "
			} else {
				stmt += " OR ( 1 "
			}
			if criteria.TxOrigin != nil {
				args = append(args, criteria.TxOrigin.Bytes())
				stmt += " AND txOrigin = ? "
			}
			if criteria.Sender != nil {
				args = append(args, criteria.Sender.Bytes())
				stmt += " AND sender = ? "
			}
			if criteria.Recipient != nil {
				args = append(args, criteria.Recipient.Bytes())
				stmt += " AND recipient = ? "
			}
			if i == length-1 {
				stmt += " )) "
			} else {
				stmt += " ) "
			}
		}
	}
	if filter.Order == DESC {
		stmt += " ORDER BY blockNumber DESC,transferIndex DESC "
	} else {
		stmt += " ORDER BY blockNumber ASC,transferIndex ASC "
	}
	if filter.Options != nil {
		stmt += " limit ?, ? "
		args = append(args, filter.Options.Offset, filter.Options.Limit)
	}
	return db.queryTransfers(ctx, stmt, args...)
}

func (db *LogDB) queryEvents(ctx context.Context, stmt string, args ...interface{}) ([]*Event, error) {
	rows, err := db.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var (
			blockID     []byte
			index       uint32
			blockNumber uint32
			blockTime   uint64
			txID        []byte
			txOrigin    []byte
			address     []byte
			topics      [5][]byte
			data        []byte
		)
		if err := rows.Scan(
			&blockID,
			&index,
			&blockNumber,
			&blockTime,
			&txID,
			&txOrigin,
			&address,
			&topics[0],
			&topics[1],
			&topics[2],
			&topics[3],
			&topics[4],
			&data,
		); err != nil {
			return nil, err
		}
		event := &Event{
			BlockID:     meter.BytesToBytes32(blockID),
			Index:       index,
			BlockNumber: blockNumber,
			BlockTime:   blockTime,
			TxID:        meter.BytesToBytes32(txID),
			TxOrigin:    meter.BytesToAddress(txOrigin),
			Address:     meter.BytesToAddress(address),
			Data:        data,
		}
		for i, topic := range topics {
			if len(topic) > 0 {
				h := meter.BytesToBytes32(topic)
				event.Topics[i] = &h
			}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (db *LogDB) queryTransfers(ctx context.Context, stmt string, args ...interface{}) ([]*Transfer, error) {
	rows, err := db.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var transfers []*Transfer
	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var (
			blockID     []byte
			index       uint32
			blockNumber uint32
			blockTime   uint64
			txID        []byte
			txOrigin    []byte
			sender      []byte
			recipient   []byte
			amount      []byte
			token       uint32
		)
		if err := rows.Scan(
			&blockID,
			&index,
			&blockNumber,
			&blockTime,
			&txID,
			&txOrigin,
			&sender,
			&recipient,
			&amount,
			&token,
		); err != nil {
			return nil, err
		}
		trans := &Transfer{
			BlockID:     meter.BytesToBytes32(blockID),
			Index:       index,
			BlockNumber: blockNumber,
			BlockTime:   blockTime,
			TxID:        meter.BytesToBytes32(txID),
			TxOrigin:    meter.BytesToAddress(txOrigin),
			Sender:      meter.BytesToAddress(sender),
			Recipient:   meter.BytesToAddress(recipient),
			Amount:      new(big.Int).SetBytes(amount),
			Token:       token,
		}
		transfers = append(transfers, trans)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return transfers, nil
}

func topicValue(topic *meter.Bytes32) []byte {
	if topic == nil {
		return nil
	}
	return topic.Bytes()
}

type BlockBatch struct {
	db        *sql.DB
	header    *block.Header
	events    []*Event
	transfers []*Transfer
}

func (bb *BlockBatch) execInTx(proc func(*sql.Tx) error) (err error) {
	tx, err := bb.db.Begin()
	if err != nil {
		return err
	}
	if err := proc(tx); err != nil {
		e := tx.Rollback()
		if e != nil {
			fmt.Println("could not rollback, error:", e)
		}

		return err
	}
	return tx.Commit()
}

func (bb *BlockBatch) Commit(abandonedBlocks ...meter.Bytes32) error {
	return bb.execInTx(func(tx *sql.Tx) error {
		for _, event := range bb.events {
			if _, err := tx.Exec("INSERT OR REPLACE INTO event(blockID ,eventIndex, blockNumber ,blockTime ,txID ,txOrigin ,address ,topic0 ,topic1 ,topic2 ,topic3 ,topic4, data) VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);",
				event.BlockID.Bytes(),
				event.Index,
				event.BlockNumber,
				event.BlockTime,
				event.TxID.Bytes(),
				event.TxOrigin.Bytes(),
				event.Address.Bytes(),
				topicValue(event.Topics[0]),
				topicValue(event.Topics[1]),
				topicValue(event.Topics[2]),
				topicValue(event.Topics[3]),
				topicValue(event.Topics[4]),
				event.Data,
			); err != nil {
				return err
			}
		}

		for _, transfer := range bb.transfers {
			if _, err := tx.Exec("INSERT OR REPLACE INTO transfer(blockID ,transferIndex, blockNumber ,blockTime ,txID ,txOrigin ,sender ,recipient ,amount, token) VALUES ( ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);",
				transfer.BlockID.Bytes(),
				transfer.Index,
				transfer.BlockNumber,
				transfer.BlockTime,
				transfer.TxID.Bytes(),
				transfer.TxOrigin.Bytes(),
				transfer.Sender.Bytes(),
				transfer.Recipient.Bytes(),
				transfer.Amount.Bytes(),
				transfer.Token,
			); err != nil {
				return err
			}
		}
		for _, id := range abandonedBlocks {
			if _, err := tx.Exec("DELETE FROM event WHERE blockID = ?;", id.Bytes()); err != nil {
				return err
			}
			if _, err := tx.Exec("DELETE FROM transfer WHERE blockID = ?;", id.Bytes()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (bb *BlockBatch) ForTransaction(txID meter.Bytes32, txOrigin meter.Address) struct {
	Insert func(tx.Events, tx.Transfers) *BlockBatch
} {
	return struct {
		Insert func(events tx.Events, transfers tx.Transfers) *BlockBatch
	}{
		func(events tx.Events, transfers tx.Transfers) *BlockBatch {
			for _, event := range events {
				bb.events = append(bb.events, newEvent(bb.header, uint32(len(bb.events)), txID, txOrigin, event))
			}
			for _, transfer := range transfers {
				bb.transfers = append(bb.transfers, newTransfer(bb.header, uint32(len(bb.transfers)), txID, txOrigin, transfer))
			}
			return bb
		},
	}
}
