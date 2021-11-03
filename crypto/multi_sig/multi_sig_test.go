// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

//
// MIT License
//
// Copyright(c) 2018 DFinlab
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

//
// This illustrates the concepts of BLS signature aggregation.
// The aggregate is only secure when the messages are distinct.
// If the messages are identical, we can make them differ by
// preppending the message with a nonce, or signer's pubkey.
//
// Th BLS go library does not currently provide an interface to
// access the public key (PublicKey.gx field).
//

package bls

import (
        "crypto/sha256"
        "fmt"

        // This is a simple C wrapper of pbc library.
        //"github.com/dfinlab/go-zdollar/crypto/multi_sig"
        "math/rand"
        "time"
)

func main() {
        messages := []string{
                "This is a message",
                "This is a message2",
                "This is a message3",
                "This is a message4",
                "This is a message5",
                "This is a message6",
                "This is a message7",
                "This is a message8",
                "This is a message9",
                "This is a message10",
        }
        params := bls.GenParamsTypeA(160, 512)
        pairing := bls.GenPairing(params)
        system, err := bls.GenSystem(pairing)
        if err != nil {
                panic(err)
        }

        N := 10

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
        hashes := make([][sha256.Size]byte, 10)
        signatures := make([]bls.Signature, 10)
        for i := 0; i < 10; i++ {
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
        indexSlice := make([]int, 10)
        for i := 0; i < 10; i++ {
                indexSlice[i] = i
        }
        rand.Seed(time.Now().UnixNano())
        rand.Shuffle(len(indexSlice), func(i, j int) {
                indexSlice[i], indexSlice[j] = indexSlice[j], indexSlice[i]
        })
        pickedIndices := indexSlice[0:6]

        // bitmap of 6 out of 10                                                                       
        fmt.Printf("%s %v\n",
                "Randomly selected 6 signature indices...",
                pickedIndices)

        // Verify each of 6
        for _, idx := range pickedIndices {
                if !bls.Verify(signatures[idx], hashes[idx], keys[idx]) {
                        panic("Unable to verify signature.")
                }
        }
        fmt.Printf("Successfully verified all 6 signatures\n")

        // Aggregate signature
        var aggregatedSignatures []bls.Signature
        var pickedHashes [][sha256.Size]byte
        var pickedKeys []bls.PublicKey
        for _, idx := range pickedIndices {
                aggregatedSignatures = append(aggregatedSignatures, signatures[idx])
                pickedHashes = append(pickedHashes, hashes[idx])
                pickedKeys = append(pickedKeys, keys[idx])
        }
        fmt.Printf("%s %v\n", "Aggregated signatures...", aggregatedSignatures)
        aggregate, err := bls.Aggregate(aggregatedSignatures, system)
        if err != nil {
                panic(err)
        }

        // Verify signature aggregate
        valid, err := bls.AggregateVerify(aggregate, pickedHashes, pickedKeys)
        if err != nil {
                panic(err)
        }

        if valid {
                fmt.Println("Signature aggregate verified!")
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

