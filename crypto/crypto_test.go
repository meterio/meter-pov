// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package crypto_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	bls "github.com/meterio/meter-pov/crypto/multi_sig"
	"github.com/meterio/meter-pov/meter"
)

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/meterio/meter-pov/crypto
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/

func TestBls(t *testing.T) {
	messages := []string{}
	N := 300
	for i := 0; i < N; i++ {
		messages = append(messages, "This is a message "+strconv.Itoa(i))
	}
	params := bls.GenParamsTypeA(160, 512)
	pairing := bls.GenPairing(params)
	system, err := bls.GenSystem(pairing)
	if err != nil {
		panic(err)
	}

	// Gene N key pairs
	keys := make([]bls.PublicKey, N)
	secrets := make([]bls.PrivateKey, N)

	for i := 0; i < N; i++ {
		keys[i], secrets[i], err = bls.GenKeys(system)
		if err != nil {
			panic(err)
		}
	}

	// Sign secrets
	hashes := make([][sha256.Size]byte, N)
	signatures := make([]bls.Signature, N)
	for i := 0; i < N; i++ {
		// TODO: will prepend pub keys[i].gx
		// the go library needs to add methods
		// to get a serialized form of  gx field.
		//
		// For now, we use 10 different messages for
		// illustration purpose.
		//
		// In either case, the final verifier has the same
		// information regarding the signed messages.
		hashes[i] = sha256.Sum256([]byte(messages[i]))
		signatures[i] = bls.Sign(hashes[i], secrets[i])
	}
	//Choose 6 by random sampling
	indexSlice := make([]int, N)
	for i := 0; i < N; i++ {
		indexSlice[i] = i
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(indexSlice), func(i, j int) {
		indexSlice[i], indexSlice[j] = indexSlice[j], indexSlice[i]
	})
	pickedIndices := indexSlice[0:N]

	// bitmap of 6 out of 10
	fmt.Printf("%s %v\n",
		"Randomly selected "+strconv.Itoa(N)+" signature indices...",
		pickedIndices)

	// Verify each of 6
	for _, idx := range pickedIndices {
		if !bls.Verify(signatures[idx], hashes[idx], keys[idx]) {
			panic("Unable to verify signature.")
		}
	}
	fmt.Printf("Successfully verified all " + strconv.Itoa(N) + " signatures\n")

	// Aggregate signature
	var aggregatedSignatures []bls.Signature
	var pickedHashes [][sha256.Size]byte
	var pickedKeys []bls.PublicKey
	for _, idx := range pickedIndices {
		aggregatedSignatures = append(aggregatedSignatures, signatures[idx])
		pickedHashes = append(pickedHashes, hashes[idx])
		pickedKeys = append(pickedKeys, keys[idx])
	}
	// fmt.Printf("%s %v\n", "Aggregated signatures...", aggregatedSignatures)
	aggregate, err := bls.Aggregate(aggregatedSignatures, system)
	if err != nil {
		panic(err)
	}

	start := time.Now()
	// Verify signature aggregate
	valid, err := bls.AggregateVerify(aggregate, pickedHashes, pickedKeys)
	if err != nil {
		panic(err)
	}

	if valid {
		fmt.Println("Signature aggregate verified!", "elapsed", meter.PrettyDuration(time.Since(start)))
	} else {
		panic("Failed to verify aggregate signature.")
	}

	// Clean up
	aggregate.Free()
	for i := 0; i < 10; i++ {
		signatures[i].Free()
		keys[i].Free()
		secrets[i].Free()
	}

	//do not need to free here
	//for i := 0; i < 6; i++ {
	//        aggregatedSignatures[i].Free()
	//}

	system.Free()
	pairing.Free()
	params.Free()
	fmt.Printf("Successfully cleaned up.\n")
}

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

func TestThresholdBLS(t *testing.T) {
	message := "this is message"
	N := 300
	params := bls.GenParamsTypeA(160, 512)
	pairing := bls.GenPairing(params)
	system, err := bls.GenSystem(pairing)
	if err != nil {
		panic(err)
	}

	// Gene N key pairs
	keys := make([]bls.PublicKey, N)
	secrets := make([]bls.PrivateKey, N)

	for i := 0; i < N; i++ {
		keys[i], secrets[i], err = bls.GenKeys(system)
		if err != nil {
			panic(err)
		}
	}

	// Sign secrets
	hash := sha256.Sum256([]byte(message))
	signatures := make([]bls.Signature, N)
	for i := 0; i < N; i++ {
		// TODO: will prepend pub keys[i].gx
		// the go library needs to add methods
		// to get a serialized form of  gx field.
		//
		// For now, we use 10 different messages for
		// illustration purpose.
		//
		// In either case, the final verifier has the same
		// information regarding the signed messages.
		signatures[i] = bls.Sign(hash, secrets[i])
	}
	//Choose 6 by random sampling
	indexSlice := make([]int, N)
	for i := 0; i < N; i++ {
		indexSlice[i] = i
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(indexSlice), func(i, j int) {
		indexSlice[i], indexSlice[j] = indexSlice[j], indexSlice[i]
	})
	pickedIndices := indexSlice[0:N]

	// bitmap of 6 out of 10
	fmt.Printf("%s %v\n",
		"Randomly selected "+strconv.Itoa(N)+" signature indices...",
		pickedIndices)

	// Verify each of 6
	for _, idx := range pickedIndices {
		if !bls.Verify(signatures[idx], hash, keys[idx]) {
			panic("Unable to verify signature.")
		}
	}
	fmt.Printf("Successfully verified all " + strconv.Itoa(N) + " signatures\n")

	// Aggregate signature
	var aggregatedSignatures []bls.Signature
	var pickedHashes [][sha256.Size]byte
	var pickedKeys []bls.PublicKey
	for _, idx := range pickedIndices {
		aggregatedSignatures = append(aggregatedSignatures, signatures[idx])
		pickedHashes = append(pickedHashes, hash)
		pickedKeys = append(pickedKeys, keys[idx])
	}
	// fmt.Printf("%s %v\n", "Aggregated signatures...", aggregatedSignatures)
	aggregate, err := bls.Aggregate(aggregatedSignatures, system)
	if err != nil {
		panic(err)
	}

	start := time.Now()
	aggregateKey, err := bls.AggregatePubkeys(keys, system)
	if err != nil {
		panic(err)
	}
	verifyValid := bls.Verify(aggregate, hash, aggregateKey)
	if verifyValid {
		fmt.Println("Successfully aggregate pubkey + verified!", "elapsed", meter.PrettyDuration(time.Since(start)))
	}

	start = time.Now()
	// Verify signature aggregate
	valid, err := bls.AggregateVerify(aggregate, pickedHashes, pickedKeys)
	if err != nil {
		panic(err)
	}

	if valid {
		fmt.Println("Signature aggregate verified!", "elapsed", meter.PrettyDuration(time.Since(start)))
	} else {
		panic("Failed to verify aggregate signature.")
	}

	// Clean up
	aggregate.Free()
	for i := 0; i < N; i++ {
		signatures[i].Free()
		keys[i].Free()
		secrets[i].Free()
	}

	//do not need to free here
	//for i := 0; i < 6; i++ {
	//        aggregatedSignatures[i].Free()
	//}

	system.Free()
	pairing.Free()
	params.Free()
	fmt.Printf("Successfully cleaned up.\n")
}
