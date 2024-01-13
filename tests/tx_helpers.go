package tests

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/script"
	"github.com/meterio/meter-pov/script/staking"
	"github.com/meterio/meter-pov/tx"
)

func BuildCallTx(chainTag byte, bestRef uint32, toAddr *meter.Address, data []byte, nonce uint64, key *ecdsa.PrivateKey) *tx.Transaction {
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 100). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(nonce)

	builder.Clause(
		tx.NewClause(toAddr).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(data),
	)
	trx := builder.Build()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	trx = trx.WithSignature(sig)
	return trx
}

func BuildMintTx(chainTag byte, bestRef uint32, to meter.Address, amount *big.Int, token byte, nonce uint64) *tx.Transaction {
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(nonce)

	builder.Clause(
		tx.NewClause(&to).WithValue(amount).WithToken(token).WithData([]byte{}),
	)
	trx := builder.Build()
	// sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	// trx = trx.WithSignature(sig)
	return trx
}

func BuildStakingTx(chainTag byte, bestRef uint32, body *staking.StakingBody, key *ecdsa.PrivateKey, nonce uint64) *tx.Transaction {
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10). //buffer for builder.Build().IntrinsicGas()
		DependsOn(nil).
		Nonce(nonce)

	data, _ := script.EncodeScriptData(body)
	builder.Clause(
		tx.NewClause(&meter.StakingModuleAddr).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(data),
	)
	trx := builder.Build()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), key)
	trx = trx.WithSignature(sig)
	return trx
}

func BuildVoteTx(chainTag byte, voterKey *ecdsa.PrivateKey, voterAddr meter.Address, candAddr meter.Address, amount *big.Int) *tx.Transaction {
	body := &staking.StakingBody{
		Opcode:     staking.OP_BOUND,
		Version:    0,
		Option:     uint32(1),
		Amount:     amount,
		HolderAddr: voterAddr,
		CandAddr:   candAddr,
		Token:      meter.MTRG,
		Timestamp:  uint64(0),
		Nonce:      0,
	}
	return BuildStakingTx(chainTag, 0, body, voterKey, 0)
}

func BuildContractCallTx(chainTag byte, bestRef uint32, to meter.Address, data []byte, signerKey *ecdsa.PrivateKey) *tx.Transaction {
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(meter.BaseTxGas * 10).
		DependsOn(nil).
		Nonce(uint64(rand.Intn(9999)))

	builder.Clause(
		tx.NewClause(&to).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(data),
	)
	trx := builder.Build()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), signerKey)
	trx = trx.WithSignature(sig)
	return trx
}

func BuildContractDeployTx(chainTag byte, bestRef uint32, data []byte, signerKey *ecdsa.PrivateKey) *tx.Transaction {
	nonce := uint64(0)
	builder := new(tx.Builder)
	builder.ChainTag(chainTag).
		BlockRef(tx.NewBlockRef(bestRef)).
		Expiration(720).
		GasPriceCoef(0).
		Gas(2200000).
		DependsOn(nil).
		Nonce(nonce)

	builder.Clause(
		tx.NewClause(nil).WithValue(big.NewInt(0)).WithToken(meter.MTRG).WithData(data),
	)
	trx := builder.Build()
	sig, _ := crypto.Sign(trx.SigningHash().Bytes(), signerKey)
	trx = trx.WithSignature(sig)
	return trx
}

func BuildCandidateTxForCand(chainTag byte, amount int) *tx.Transaction {
	body := &staking.StakingBody{
		Opcode:          staking.OP_CANDIDATE,
		Version:         0,
		Option:          uint32(0),
		Amount:          BuildAmount(amount),
		HolderAddr:      CandAddr,
		CandAddr:        CandAddr,
		CandName:        CandName,
		CandDescription: CandDesc,
		CandPubKey:      CandPubKey,
		CandPort:        CandPort,
		CandIP:          CandIP,
		Token:           meter.MTRG,
		Timestamp:       uint64(0),
		Nonce:           0,
	}
	return BuildStakingTx(chainTag, 0, body, CandKey, 0)
}

func BuildCandidateTxForCand2(chainTag byte, amount int) *tx.Transaction {
	body := &staking.StakingBody{
		Opcode:          staking.OP_CANDIDATE,
		Version:         0,
		Option:          uint32(0),
		Amount:          BuildAmount(amount),
		HolderAddr:      Cand2Addr,
		CandAddr:        Cand2Addr,
		CandName:        Cand2Name,
		CandDescription: Cand2Desc,
		CandPubKey:      Cand2PubKey,
		CandPort:        Cand2Port,
		CandIP:          Cand2IP,
		Token:           meter.MTRG,
		Timestamp:       uint64(0),
		Nonce:           0,
	}
	return BuildStakingTx(chainTag, 0, body, Cand2Key, 0)
}
