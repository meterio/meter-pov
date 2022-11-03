package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/trie"
	"gopkg.in/urfave/cli.v1"
	"os"
)

func importSnapshotAction(ctx *cli.Context) error {
	initLogger()

	mainDB, _ := openMainDB(ctx)
	defer func() { log.Info("closing main database..."); mainDB.Close() }()

	blkNumber := ctx.String(revisionFlag.Name)

	snap := trie.NewStateSnapshot()
	dbDir := ctx.String(dataDirFlag.Name)
	prefix := fmt.Sprintf("%v/state-snap-%v", dbDir, blkNumber)
	stateBloom, err := trie.NewStateBloomFromDisk(prefix + ".bloom")
	if err != nil {
		//	...
	}
	snap.Bloom = stateBloom

	// -----------------------------------------
	dat, err := os.ReadFile(prefix + "-nodes.db")
	if err != nil {

	}
	buf := bytes.NewBuffer(dat)
	dec := gob.NewDecoder(buf)

	nodes := make(map[string][]byte)
	err = dec.Decode(&nodes)
	if err != nil {

	}
	for s, value := range nodes {
		key, _ := hex.DecodeString(s)
		mainDB.Put(key, value)
	}

	log.Info("loaded snapshot", "file", prefix+"-nodes.db")

	// -----------------------------------------
	dat, err = os.ReadFile(prefix + "-codes.db")
	if err != nil {

	}
	buf = bytes.NewBuffer(dat)
	dec = gob.NewDecoder(buf)

	codes := make(map[string][]byte)
	err = dec.Decode(&codes)
	if err != nil {

	}

	for s, value := range codes {
		key, _ := hex.DecodeString(s)
		mainDB.Put(key, value)
	}

	log.Info("loaded snapshot", "file", prefix+"-codes.db")

	// -----------------------------------------
	dat, err = os.ReadFile(prefix + "-leaf-keys.db")
	if err != nil {

	}
	buf = bytes.NewBuffer(dat)
	dec = gob.NewDecoder(buf)

	leafKeys := make(map[string][]byte)
	err = dec.Decode(&leafKeys)
	if err != nil {

	}

	for s, value := range leafKeys {
		key, _ := hex.DecodeString(s)
		mainDB.Put(key, value)

		//if err := rlp.DecodeBytes(value, &stateAcc); err != nil {
		//	fmt.Println("Invalid account encountered during traversal", "err", err)
		//	continue
		//}
	}

	log.Info("loaded snapshot", "file", prefix+"-leaf-keys.db")

	// -----------------------------------------
	dat, err = os.ReadFile(prefix + "-leaf-blobs.db")
	if err != nil {

	}
	buf = bytes.NewBuffer(dat)
	dec = gob.NewDecoder(buf)

	leafBlobs := make(map[string][]byte)
	err = dec.Decode(&leafBlobs)
	if err != nil {

	}

	var stateAcc trie.StateAccount
	for s, value := range leafBlobs {
		key, _ := hex.DecodeString(s)
		mainDB.Put(key, value)

		if err := rlp.DecodeBytes(value, &stateAcc); err != nil {
			fmt.Println("Invalid account encountered during traversal", "err", err)
			continue
		}
	}

	log.Info("loaded snapshot", "file", prefix+"-leaf-blobs.db")

	// -----------------------------------------
	dat, err = os.ReadFile(prefix + "-storage-nodes.db")
	if err != nil {

	}
	buf = bytes.NewBuffer(dat)
	dec = gob.NewDecoder(buf)

	nodeStorages := make(map[string][]byte)
	err = dec.Decode(&nodeStorages)
	if err != nil {

	}
	for s, value := range nodeStorages {
		key, _ := hex.DecodeString(s)
		mainDB.Put(key, value)
	}

	log.Info("loaded snapshot", "file", prefix+"-storage-nodes.db")

	// -----------------------------------------
	dat, err = os.ReadFile(prefix + "-storage-leaf-keys.db")
	if err != nil {

	}
	buf = bytes.NewBuffer(dat)
	dec = gob.NewDecoder(buf)

	storageLeafKeys := make(map[string][]byte)
	err = dec.Decode(&storageLeafKeys)
	if err != nil {

	}

	for s, value := range storageLeafKeys {
		key, _ := hex.DecodeString(s)
		mainDB.Put(key, value)
	}

	log.Info("loaded snapshot", "file", prefix+"-storage-leaf-keys.db")

	// -----------------------------------------
	dat, err = os.ReadFile(prefix + "-storage-leaf-blobs.db")
	if err != nil {

	}
	buf = bytes.NewBuffer(dat)
	dec = gob.NewDecoder(buf)

	storageLeafBlobs := make(map[string][]byte)
	err = dec.Decode(&storageLeafBlobs)
	if err != nil {

	}

	for s, value := range storageLeafBlobs {
		key, _ := hex.DecodeString(s)
		mainDB.Put(key, value)
	}

	log.Info("loaded snapshot", "file", prefix+"-storage-leaf-blobs.db")

	// -----------------------------------------
	dat, err = os.ReadFile(prefix + "-root-enc.db")
	if err != nil {

	}
	buf = bytes.NewBuffer(dat)
	dec = gob.NewDecoder(buf)

	rootEnc := make(map[string][]byte)
	err = dec.Decode(&rootEnc)
	if err != nil {

	}

	var blkStateRoot meter.Bytes32
	for s, value := range rootEnc {
		key, _ := hex.DecodeString(s)
		copy(blkStateRoot[:], key[:])
		mainDB.Put(key, value)
	}

	log.Info("loaded snapshot", "file", prefix+"-root-enc.db")

	return nil
}
