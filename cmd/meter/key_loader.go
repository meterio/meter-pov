// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/dfinlab/meter/consensus"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/ethereum/go-ethereum/crypto"
	cli "gopkg.in/urfave/cli.v1"
)

// fileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func verifyECDSA(privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) bool {
	hash := []byte("testing")
	r, s, err := ecdsa.Sign(strings.NewReader("test-plain-text-some-thing"), privKey, hash)
	if err != nil {
		fmt.Println("Error during sign: ", err)
		return false
	}
	return ecdsa.Verify(pubKey, hash, r, s)
}

func verifyBls(privKey bls.PrivateKey, pubKey bls.PublicKey) bool {
	signMsg := string("This is a test")
	msgHash := sha256.Sum256([]byte(signMsg))
	sig := bls.Sign(msgHash, privKey)
	valid := bls.Verify(sig, msgHash, pubKey)
	return valid
}

type KeyLoader struct {
	masterPath   string
	publicPath   string
	masterBytes  []byte
	publicBytes  []byte
	ecdsaPrivKey *ecdsa.PrivateKey
	ecdsaPubKey  *ecdsa.PublicKey
	blsPrivKey   *bls.PrivateKey
	blsPubKey    *bls.PublicKey

	updated bool
}

func NewKeyLoader(ctx *cli.Context) *KeyLoader {
	masterPath := masterKeyPath(ctx)
	publicPath := publicKeyPath(ctx)
	var masterBytes, publicBytes []byte
	if fileExists(masterPath) {
		masterBytes, _ = ioutil.ReadFile(masterPath)
		masterBytes = []byte(strings.TrimSuffix(string(masterBytes), "\n"))
	}
	if fileExists(publicPath) {
		publicBytes, _ = ioutil.ReadFile(publicPath)
		publicBytes = []byte(strings.TrimSuffix(string(publicBytes), "\n"))
	}
	return &KeyLoader{
		masterPath:  masterPath,
		publicPath:  publicPath,
		masterBytes: masterBytes,
		publicBytes: publicBytes,

		updated: false,
	}
}

func (k *KeyLoader) genECDSA() error {
	k.updated = true
	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Println("Error during ECDSA key pair generation: ", err)
		return err
	}
	k.ecdsaPrivKey = key
	k.ecdsaPubKey = &key.PublicKey
	return nil
}

func (k *KeyLoader) validateECDSA() error {
	split := strings.Split(string(k.masterBytes), ":::")
	if len(split) == 0 {
		return k.genECDSA()
	}
	privBytes, err := b64.StdEncoding.DecodeString(split[0])
	if err != nil {
		return k.genECDSA()
	}
	privKey, err := crypto.ToECDSA(privBytes)
	if err != nil {
		return k.genECDSA()
	}
	k.ecdsaPrivKey = privKey
	psplit := strings.Split(string(k.publicBytes), ":::")
	if len(split) == 0 {
		k.ecdsaPubKey = &k.ecdsaPrivKey.PublicKey
		return nil
	}
	pubBytes, err := b64.StdEncoding.DecodeString(psplit[0])
	if err != nil {
		k.ecdsaPubKey = &k.ecdsaPrivKey.PublicKey
		return nil
	}
	pubKey, err := crypto.UnmarshalPubkey(pubBytes)
	if err != nil || !verifyECDSA(privKey, pubKey) {
		k.ecdsaPubKey = &k.ecdsaPrivKey.PublicKey
	} else {
		k.ecdsaPubKey = pubKey
	}
	return nil
}

func (k *KeyLoader) genBls() error {
	system, err := getBlsSystem()
	if err != nil {
		return err
	}

	k.updated = true
	pubKey, privKey, err := bls.GenKeys(*system)
	if err != nil {
		return err
	}
	k.blsPrivKey = &privKey
	k.blsPubKey = &pubKey
	return nil
}

func (k *KeyLoader) validateBls() error {
	split := strings.Split(string(k.masterBytes), ":::")
	psplit := strings.Split(string(k.publicBytes), ":::")
	if len(split) < 2 || len(psplit) < 2 {
		return k.genBls()
	}
	system, err := getBlsSystem()
	if err != nil {
		return err
	}
	privBytes, err := b64.StdEncoding.DecodeString(split[1])
	if err != nil {
		return k.genBls()
	}
	pubBytes, err := b64.StdEncoding.DecodeString(psplit[1])
	if err != nil {
		return k.genBls()
	}
	privKey, err := system.PrivKeyFromBytes(privBytes)
	if err != nil {
		return k.genBls()
	}
	pubKey, err := system.PubKeyFromBytes(pubBytes)
	if err != nil || !verifyBls(privKey, pubKey) {
		return k.genBls()
	}
	k.blsPrivKey = &privKey
	k.blsPubKey = &pubKey
	return nil
}

func (k *KeyLoader) saveKeys(system bls.System) error {
	ecdsaPrivBytes := crypto.FromECDSA(k.ecdsaPrivKey)
	ecdsaPrivB64 := b64.StdEncoding.EncodeToString(ecdsaPrivBytes)
	ecdsaPubBytes := crypto.FromECDSAPub(k.ecdsaPubKey)
	ecdsaPubB64 := b64.StdEncoding.EncodeToString(ecdsaPubBytes)
	blsPrivBytes := system.PrivKeyToBytes(*k.blsPrivKey)
	blsPrivB64 := b64.StdEncoding.EncodeToString(blsPrivBytes)
	blsPubBytes := system.PubKeyToBytes(*k.blsPubKey)
	blsPubB64 := b64.StdEncoding.EncodeToString(blsPubBytes)

	priv := strings.Join([]string{ecdsaPrivB64, blsPrivB64}, ":::")
	pub := strings.Join([]string{ecdsaPubB64, blsPubB64}, ":::")
	k.masterBytes = []byte(priv)
	k.publicBytes = []byte(pub)
	err := ioutil.WriteFile(k.masterPath, []byte(priv+"\n"), 0600)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(k.publicPath, []byte(pub+"\n"), 0600)
	return err
}

func (k *KeyLoader) Load() (*ecdsa.PrivateKey, *ecdsa.PublicKey, *consensus.BlsCommon, error) {
	err := k.validateECDSA()
	if err != nil {
		fmt.Println("could not validate ecdsa keys, error:", err)
		panic("could not validate ecdsa keys")
	}

	err = k.validateBls()
	if err != nil {
		fmt.Println("could not validate ecdsa keys, error:", err)
		panic("could not validate ecdsa keys")
	}

	paraBytes, err := hex.DecodeString(paraString)
	if err != nil {
		fmt.Println("decode paraString error:", err)
		return nil, nil, nil, nil
	}
	params, err := bls.ParamsFromBytes(paraBytes)
	if err != nil {
		fmt.Println("params from bytes error:", err)
		return nil, nil, nil, nil
	}
	pairing := bls.GenPairing(params)

	systemBytes, err := hex.DecodeString(systemString)
	if err != nil {
		fmt.Println("decode system string error:", err)
		return nil, nil, nil, nil
	}

	system, err := bls.SystemFromBytes(pairing, systemBytes)
	if err != nil {
		fmt.Println("system from bytes error:", err)
		return nil, nil, nil, nil
	}

	if k.updated == true {
		err := k.saveKeys(system)
		if err != nil {
			fmt.Println("save keys error:", err)
		}

	}

	blsCommon := consensus.NewBlsCommonFromParams(*k.blsPubKey, *k.blsPrivKey, system, params, pairing)
	return k.ecdsaPrivKey, k.ecdsaPubKey, blsCommon, nil
}
