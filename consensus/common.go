package consensus

import (
	"bytes"
	sha256 "crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"path/filepath"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
)

const (
	TYPE_A_SIGNATRUE_SIZE = int(65)

	INITIALIZE_AS_LEADER           = int(1)
	INITIALIZE_AS_VALIDATOR        = int(2)
	INITIALIZE_AS_REPLAY_LEADER    = int(3)
	INITIALIZE_AS_REPLAY_VALIDATOR = int(4)
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

func NewConsensusCommonFromBlsCommon(conR *ConsensusReactor, blsCommon *BlsCommon) *ConsensusCommon {
	return &ConsensusCommon{
		PrivKey:     blsCommon.PrivKey,
		PubKey:      blsCommon.PubKey,
		csReactor:   conR,
		system:      blsCommon.system,
		params:      blsCommon.params,
		pairing:     blsCommon.pairing,
		initialized: true,
		initialRole: INITIALIZE_AS_LEADER,
	}
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
		cc.pairing.Free()
	} else if cc.initialRole == INITIALIZE_AS_REPLAY_LEADER {
		cc.pairing.Free()
	} else if cc.initialRole == INITIALIZE_AS_REPLAY_VALIDATOR {
		cc.pairing.Free()
	}

	cc.initialized = false
	return true
}

func (cc *ConsensusCommon) GetSystem() *bls.System {
	return &cc.system
}

func (cc *ConsensusCommon) GetParams() *bls.Params {
	return &cc.params
}

func (cc *ConsensusCommon) GetPairing() *bls.Pairing {
	return &cc.pairing
}

func (cc *ConsensusCommon) GetPublicKey() *bls.PublicKey {
	return &cc.PubKey
}

func (cc *ConsensusCommon) GetPrivateKey() *bls.PrivateKey {
	return &cc.PrivKey
}

// sign the part of msg
func (cc *ConsensusCommon) Hash256Msg(msg []byte) [32]byte {
	return sha256.Sum256(msg)
}

// sign the part of msg
func (cc *ConsensusCommon) SignMessage(msg []byte) (bls.Signature, [32]byte) {
	hash := sha256.Sum256(msg)
	sig := bls.Sign(hash, cc.PrivKey)
	return sig, hash
}

// the return with slice byte
func (cc *ConsensusCommon) SignMessage2(msg []byte) ([]byte, [32]byte) {
	hash := sha256.Sum256(msg)
	sig := bls.Sign(hash, cc.PrivKey)
	return cc.system.SigToBytes(sig), hash
}

func (cc *ConsensusCommon) VerifySignature(signature, msgHash, blsPK []byte) bool {
	var fixedMsgHash [32]byte
	copy(fixedMsgHash[:], msgHash[32:])
	pubkey, err := cc.system.PubKeyFromBytes(blsPK)
	if err != nil {
		fmt.Println("pubkey unmarshal failed")
		return false
	}

	sig, err := cc.system.SigFromBytes(signature)
	if err != nil {
		fmt.Println("signature unmarshal failed")
		return false
	}
	return bls.Verify(sig, fixedMsgHash, pubkey)
}

func (cc *ConsensusCommon) AggregateSign(sigs []bls.Signature) bls.Signature {
	sig, err := bls.Aggregate(sigs, cc.system)
	if err != nil {
		fmt.Println("aggreate signature failed")
	}
	return sig
}

func (cc *ConsensusCommon) AggregateSign2(sigs []bls.Signature) []byte {
	sig, err := bls.Aggregate(sigs, cc.system)
	if err != nil {
		fmt.Println("aggreate signature failed")
	}
	return cc.system.SigToBytes(sig)
}

// all voter sign the same msg.
func (cc *ConsensusCommon) AggregateVerify(sig bls.Signature, hashes [][32]byte, pubKeys []bls.PublicKey) (bool, error) {
	return bls.AggregateVerify(sig, hashes, pubKeys)
}

// backup BLS pubkey/privkey into file in case of replay, this backup happens every committee establishment (30mins)
func writeOutKeyPairs(conR *ConsensusReactor, system bls.System, pubKey bls.PublicKey, privKey bls.PrivateKey) error {
	// write pub/pri key to file
	pubBytes := system.PubKeyToBytes(pubKey)
	privBytes := system.PrivKeyToBytes(privKey)

	isolator := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	content := make([]byte, 0)
	content = append(content, pubBytes...)
	content = append(content, isolator...)
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
	ioutil.WriteFile(filepath.Join(conR.dataDir, "consensus.key"), []byte(hex.EncodeToString(content)), 0644)
	return nil
}

func readBackKeyPairs(conR *ConsensusReactor, system bls.System) (*bls.PublicKey, *bls.PrivateKey, error) {
	readBytes, err := ioutil.ReadFile(filepath.Join(conR.dataDir, "consensus.key"))
	if err != nil {
		conR.logger.Error("read consesus.key file error ...")
		return nil, nil, err
	}

	keyBytes, err := hex.DecodeString(string(readBytes))
	if err != nil {
		conR.logger.Error("convert hex error ...")
		return nil, nil, err
	}

	isolator := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	split := bytes.Split(keyBytes, isolator)

	pubBytes := split[0]
	privBytes := split[1]

	/****
	fmt.Println("Pub", pubBytes)
	fmt.Println("Priv", privBytes)
	****/

	pubKey, err := system.PubKeyFromBytes(pubBytes)
	if err != nil {
		conR.logger.Error("read pubKey error ...")
		return nil, nil, err
	}

	privKey, err := system.PrivKeyFromBytes(privBytes)
	if err != nil {
		conR.logger.Error("read privKey error ...")
		return nil, nil, err
	}

	return &pubKey, &privKey, nil
}
