package consensus

import (
	//"bytes"
	//    "errors"
	"fmt"
	//"time"
	sha256 "crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"path/filepath"

	bls "github.com/vechain/thor/crypto/multi_sig"
)

const (
	TYPE_A_SIGNATRUE_SIZE = int(65)

	INITIALIZE_AS_LEADER    = int(1)
	INITIALIZE_AS_VALIDATOR = int(2)
)

type ConsensusCommon struct {
	PrivKey   bls.PrivateKey    //my private key
	PubKey    bls.PublicKey     //my public key
	csReactor *ConsensusReactor //global reactor info

	//global params of BLS
	system      bls.System
	params      bls.Params
	pairing     bls.Pairing
	initialized bool
	initialRole int
}

// XXX: currently Type A is being used
// Committee Leader calls NewConsensusCommon and distributes system and params out
// by AnnounceCommittee. Validators receive AnnounceCommittee call
// NewValidatorConsensusCommon to initialize by receiving system and params

func NewConsensusCommon(conR *ConsensusReactor) *ConsensusCommon {
	//var cc *ConsensusCommon

	params := bls.GenParamsTypeA(160, 512)
	pairing := bls.GenPairing(params)
	system, err := bls.GenSystem(pairing)
	if err != nil {
		panic(err)
	}

	PubKey, PrivKey, err := bls.GenKeys(system)
	if err != nil {
		panic(err)
	}
	// XXX backup to file

	writeOutKeyPairs(conR, system, PubKey, PrivKey)

	return &ConsensusCommon{
		PrivKey:     PrivKey,
		PubKey:      PubKey,
		csReactor:   conR,
		system:      system,
		params:      params,
		pairing:     pairing,
		initialized: true,
		initialRole: INITIALIZE_AS_LEADER,
	}
}

// Validator receives paramBytes, systemBytes from Leader and generate key pair
func NewValidatorConsensusCommon(conR *ConsensusReactor, paramBytes []byte, systemBytes []byte) *ConsensusCommon {
	params, err := bls.ParamsFromBytes(paramBytes)
	if err != nil {
		fmt.Println("initialize param failed...")
		panic(err)
	}

	pairing := bls.GenPairing(params)
	system, err := bls.SystemFromBytes(pairing, systemBytes)
	if err != nil {
		fmt.Println("initialize system failed...")
		panic(err)
	}

	PubKey, PrivKey, err := bls.GenKeys(system)
	if err != nil {
		panic(err)
	}
	//backup to file

	return &ConsensusCommon{
		PrivKey:     PrivKey,
		PubKey:      PubKey,
		csReactor:   conR,
		system:      system,
		params:      params,
		pairing:     pairing,
		initialized: true,
		initialRole: INITIALIZE_AS_VALIDATOR,
	}
}

// Leader in replay mode should use existed paramBytes, systemBytes  and generate key pair
func NewReplayLeaderConsensusCommon(conR *ConsensusReactor, paramBytes []byte, systemBytes []byte) *ConsensusCommon {
	params, err := bls.ParamsFromBytes(paramBytes)
	if err != nil {
		fmt.Println("initialize param failed...")
		panic(err)
	}

	pairing := bls.GenPairing(params)
	system, err := bls.SystemFromBytes(pairing, systemBytes)
	if err != nil {
		fmt.Println("initialize system failed...")
		panic(err)
	}

	// XXX: read from file
	/****
	PubKey, PrivKey, err := bls.GenKeys(system)
	if err != nil {
		panic(err)
	}
	***/
	PrivKey := bls.PrivateKey{}
	PubKey := bls.PublicKey{}

	return &ConsensusCommon{
		PrivKey:     PrivKey,
		PubKey:      PubKey,
		csReactor:   conR,
		system:      system,
		params:      params,
		pairing:     pairing,
		initialized: true,
		initialRole: INITIALIZE_AS_LEADER,
	}
}

// Validator receives paramBytes, systemBytes from Leader and generate key pair
func NewValidatorReplayConsensusCommon(conR *ConsensusReactor, paramBytes []byte, systemBytes []byte) *ConsensusCommon {
	params, err := bls.ParamsFromBytes(paramBytes)
	if err != nil {
		fmt.Println("initialize param failed...")
		panic(err)
	}

	pairing := bls.GenPairing(params)
	system, err := bls.SystemFromBytes(pairing, systemBytes)
	if err != nil {
		fmt.Println("initialize system failed...")
		panic(err)
	}

	// read from file
	/***********
	PubKey, PrivKey, err := bls.GenKeys(system)
	if err != nil {
		panic(err)
	}
	**************/
	PrivKey := bls.PrivateKey{}
	PubKey := bls.PublicKey{}

	writeOutKeyPairs(conR, system, PubKey, PrivKey)

	return &ConsensusCommon{
		PrivKey:     PrivKey,
		PubKey:      PubKey,
		csReactor:   conR,
		system:      system,
		params:      params,
		pairing:     pairing,
		initialized: true,
		initialRole: INITIALIZE_AS_VALIDATOR,
	}
}

func writeOutKeyPairs(conR *ConsensusReactor, system bls.System, pubKey bls.PublicKey, privKey bls.PrivateKey) {
	// write pub/pri key to file
	pubBytes := system.PubKeyToBytes(pubKey)
	privBytes := system.PrivKeyToBytes(privKey)
	content := make([]byte, 0)
	content = append(content, pubBytes...)
	content = append(content, []byte("\n")...)
	content = append(content, privBytes...)
	/*
		var hash [sha256.Size]byte
		fmt.Println("PUB KEY BYTES: ", pubBytes)
		fmt.Println("PRIV KEY BYTES: ", privBytes)

		pk, err := system.PubKeyFromBytes(pubBytes)
		rk, err := system.PrivKeyFromBytes(privBytes)
		s := bls.Sign(hash, rk)
		result := bls.Verify(s, hash, pk)
		fmt.Println("VERIFY RESULT:    ", result)
		content := system.PubKeyToBytes(PubKey)
		content = append(content, ([]byte("\n"))...)
		content = append(content, system.PrivKeyToBytes(PrivKey)...)
	*/
	ioutil.WriteFile(filepath.Join(conR.ConfigPath, "consensus.key"), []byte(hex.EncodeToString(content)), 0644)
}

// BLS is implemented by C, memeory need to be freed.
// Signatures also need to be freed but Not here!!!
func (cc *ConsensusCommon) ConsensusCommonDeinit() bool {
	if !cc.initialized {
		fmt.Println("BLS is not initialized!")
		return false
	}

	if cc.initialRole == INITIALIZE_AS_LEADER {
		cc.PubKey.Free()
		cc.PrivKey.Free()
		cc.system.Free()
		cc.pairing.Free()
		cc.params.Free()
	} else if cc.initialRole == INITIALIZE_AS_VALIDATOR {
		cc.PubKey.Free()
		cc.PrivKey.Free()
	}

	cc.initialized = false
	return true
}

func (cc *ConsensusCommon) checkConsensusCommonInit() {
	if !cc.initialized {
		fmt.Println("BLS is not initialized!")
	}
}

// sign the part of msg
func (cc *ConsensusCommon) Hash256Msg(msg []byte, offset uint32, length uint32) [32]byte {

	cc.checkConsensusCommonInit()

	return sha256.Sum256(msg[offset : offset+length])
}

// sign the part of msg
func (cc *ConsensusCommon) SignMessage(msg []byte, offset uint32, length uint32) bls.Signature {

	cc.checkConsensusCommonInit()
	//hash := crypto.Sha256(msg[offset : offset+length])
	hash := sha256.Sum256(msg[offset : offset+length])
	sig := bls.Sign(hash, cc.PrivKey)
	return sig
}

// the return with slice byte
func (cc *ConsensusCommon) SignMessage2(msg []byte, offset uint32, length uint32) []byte {
	cc.checkConsensusCommonInit()
	//hash := crypto.Sha256(msg[offset : offset+length])
	hash := sha256.Sum256(msg[offset : offset+length])
	sig := bls.Sign(hash, cc.PrivKey)
	return cc.system.SigToBytes(sig)
}

//verify the signature in message
func (cc *ConsensusCommon) VerifyMessage(msg []byte, offset uint32, length uint32) bool {
	cc.checkConsensusCommonInit()
	//hash := crypto.Sha256(msg[offset : offset+length])
	hash := sha256.Sum256(msg[offset : offset+length])
	sig, err := cc.system.SigFromBytes(msg[offset : offset+length])
	if err != nil {
		panic(err)
	}
	verify := bls.Verify(sig, hash, cc.PubKey)
	if verify != true {
		fmt.Println("verify signature failed")
	}
	return verify
}

func (cc *ConsensusCommon) AggregateSign(sigs []bls.Signature) bls.Signature {
	cc.checkConsensusCommonInit()
	sig, err := bls.Aggregate(sigs, cc.system)
	if err != nil {
		fmt.Println("aggreate signature failed")
	}
	return sig
}

func (cc *ConsensusCommon) AggregateSign2(sigs []bls.Signature) []byte {
	cc.checkConsensusCommonInit()
	sig, err := bls.Aggregate(sigs, cc.system)
	if err != nil {
		fmt.Println("aggreate signature failed")
	}
	return cc.system.SigToBytes(sig)
}

// all voter sign the same msg.
func (cc *ConsensusCommon) AggregateVerify(sig bls.Signature, hashes [][32]byte, pubKeys []bls.PublicKey) (bool, error) {
	cc.checkConsensusCommonInit()
	return bls.AggregateVerify(sig, hashes, pubKeys)
}
