package main_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/enzoh/go-bls"
)

func TestBlsKey(t *testing.T) {
	system, err := getBlsSystem()
	if err != nil {
		return err
	}

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
