// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package crypto_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"

	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/ethereum/go-ethereum/crypto"
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/crypto
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/

func TestBlsSig(t *testing.T) {
	fmt.Println("Test for BLS Signature")
	params := bls.GenParamsTypeA(160, 512)
	pairing := bls.GenPairing(params)
	system, err := bls.GenSystem(pairing)
	if err != nil {
		fmt.Println("ERROR: ", err)
		t.Fail()
	}
	_, sk, err := bls.GenKeys(system)
	if err != nil {
		fmt.Println("Could not generate keys:", err)
	}

	msgHash := sha256.Sum256([]byte("This is a message to be signed"))
	msgHashB64 := base64.StdEncoding.EncodeToString(msgHash[:])
	fmt.Println("MsgHash B64: ", msgHashB64)
	for i := 0; i < 10; i++ {
		sign := bls.Sign(msgHash, sk)
		signBytes := system.SigToBytes(sign)
		signB64 := base64.StdEncoding.EncodeToString(signBytes)
		fmt.Println("#", i, " : Signature B64: ", signB64)
	}
}

func TestECDSASig(t *testing.T) {
	fmt.Println("Test for ECDSA Signature")
	sk, err := crypto.GenerateKey()
	if err != nil {
		fmt.Println("Could not generate ECDSA keys: ", err)
		t.Fail()
	}
	msgHash := sha256.Sum256([]byte("This is a message to be signed"))
	msgHashB64 := base64.StdEncoding.EncodeToString(msgHash[:])
	fmt.Println("MsgHash B64: ", msgHashB64)
	sigs := make(map[string]bool)
	for i := 0; i < 100000; i++ {
		signBytes, err := crypto.Sign(msgHash[:], sk)
		if err != nil {
			fmt.Println("Could not sign:", err)
			t.Fail()
		}
		signB64 := base64.StdEncoding.EncodeToString(signBytes)
		if _, tracked := sigs[signB64]; !tracked {
			fmt.Println("#", i, " : Signature B64: ", signB64)
			sigs[signB64] = true
		}
	}
}
