// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/meterio/meter-pov/kv"
	"github.com/meterio/meter-pov/lvldb"
	"github.com/meterio/meter-pov/meter"
)

func fatal(args ...interface{}) {
	var w io.Writer
	if runtime.GOOS == "windows" {
		// The SameFile check below doesn't work on Windows.
		// stdout is unlikely to get redirected though, so just print there.
		w = os.Stdout
	} else {
		outf, err := os.Stdout.Stat()
		if err != nil {
			fmt.Println("could not get os stdout, error:", err)
			panic("could not get os stdout")
		}

		errf, err := os.Stderr.Stat()
		if err != nil {
			fmt.Println("could not get os stderr, error:", err)
			panic("could not get os stderr")
		}

		if outf != nil && errf != nil && os.SameFile(outf, errf) {
			w = os.Stderr
		} else {
			w = io.MultiWriter(os.Stdout, os.Stderr)
		}
	}
	fmt.Fprint(w, "Fatal: ")
	fmt.Fprintln(w, args...)
	os.Exit(1)
}

func handleExitSignal() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		exitSignalCh := make(chan os.Signal)
		signal.Notify(exitSignalCh, os.Interrupt, os.Kill, syscall.SIGTERM)

		select {
		case sig := <-exitSignalCh:
			log.Info("exit signal received", "signal", sig)
			cancel()
		}
	}()
	return ctx
}

// ReadTrieNode retrieves the trie node of the provided hash.
func ReadTrieNode(db kv.Getter, hash meter.Bytes32) []byte {
	data, _ := db.Get(hash.Bytes())
	return data
}

// HasCode checks if the contract code corresponding to the
// provided code hash is present in the db.
func HasCode(db kv.Getter, hash meter.Bytes32) bool {
	// Try with the prefixed code scheme first, if not then try with legacy
	// scheme.
	// if ok := HasCodeWithPrefix(db, hash); ok {
	// 	return true
	// }
	ok, _ := db.Has(hash.Bytes())
	return ok
}

func getJson(client *http.Client, url string, target interface{}) error {
	r, err := client.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func getValue(ldb *lvldb.LevelDB, key []byte) (string, error) {
	val, err := ldb.Get(key)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(val), err
}

func updateLeaf(ldb *lvldb.LevelDB, hexVal string, dryRun bool) error {
	v, _ := hex.DecodeString(hexVal)
	if !dryRun {
		ldb.Put(leafBlockKey, v)
		fmt.Println("leaf updated:", hexVal)
	} else {
		fmt.Println("leaf will be:", hexVal)
	}
	return nil
}

func readLeaf(ldb *lvldb.LevelDB) (string, error) {
	val, err := getValue(ldb, leafBlockKey)
	return val, err
}

func updateBestQC(ldb *lvldb.LevelDB, hexVal string, dryRun bool) error {
	v, _ := hex.DecodeString(hexVal)
	if !dryRun {
		ldb.Put(bestQCKey, v)
		fmt.Println("best-qc updated:", hexVal)
	} else {
		fmt.Println("best-qc will be:", hexVal)
	}
	return nil
}

func readBestQC(ldb *lvldb.LevelDB) (string, error) {
	val, err := getValue(ldb, bestQCKey)
	return val, err
}

func updateBest(ldb *lvldb.LevelDB, hexVal string, dryRun bool) error {
	v, _ := hex.DecodeString(hexVal)
	if !dryRun {
		ldb.Put(bestBlockKey, v)
		fmt.Println("best updated:", hexVal)
	} else {
		fmt.Println("best will be:", hexVal)
	}
	return nil
}

func readBest(ldb *lvldb.LevelDB) (string, error) {
	val, err := getValue(ldb, bestBlockKey)
	return val, err
}

// loadFlatternIndexStart returns the best block ID on trunk.

func loadPruneIndexHead(r kv.Getter) (uint32, error) {
	data, err := r.Get(pruneIndexHeadKey)
	if err != nil {
		return 0, err
	}
	num := binary.LittleEndian.Uint32(data)
	return num, nil
}

func savePruneIndexHead(w kv.Putter, num uint32) error {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, num)
	return w.Put(pruneIndexHeadKey, b)
}

func loadPruneStateHead(r kv.Getter) (uint32, error) {
	data, err := r.Get(pruneStateHeadKey)
	if err != nil {
		return 0, err
	}
	num := binary.LittleEndian.Uint32(data)
	return num, nil
}

func savePruneStateHead(w kv.Putter, num uint32) error {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, num)
	return w.Put(pruneStateHeadKey, b)
}

func loadBestBlockIDBeforeFlattern(r kv.Getter) (meter.Bytes32, error) {
	data, err := r.Get(bestBeforeFlatternKey)
	if err != nil {
		return meter.Bytes32{}, err
	}
	return meter.BytesToBytes32(data), nil
}

func saveBestBlockIDBeforeFlattern(w kv.Putter, id meter.Bytes32) error {
	return w.Put(bestBeforeFlatternKey, id.Bytes())
}

func GenECDSAKeys() (*ecdsa.PublicKey, *ecdsa.PrivateKey, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Println("Error during ECDSA key pair generation: ", err)
		return nil, nil, err
	}
	return &key.PublicKey, key, nil
}
