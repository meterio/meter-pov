package consensus

import (
	sha256 "crypto/sha256"
	"fmt"

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
