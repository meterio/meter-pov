// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

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

// PrettyDuration is a pretty printed version of a time.Duration value that cuts
// the unnecessary precision off from the formatted textual representation.
type PrettyDuration time.Duration

var prettyDurationRe = regexp.MustCompile(`\.[0-9]{4,}`)

// String implements the Stringer interface, allowing pretty printing of duration
// values rounded to three decimals.
func (d PrettyDuration) String() string {
	label := time.Duration(d).String()
	if match := prettyDurationRe.FindString(label); len(match) > 4 {
		label = strings.Replace(label, match, match[:4], 1)
	}
	return label
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
	leafBlockKey := []byte("leaf")
	v, _ := hex.DecodeString(hexVal)
	if dryRun {
		ldb.Put(leafBlockKey, v)
		fmt.Println("leaf updated:", hexVal)
	} else {
		fmt.Println("leaf will be:", hexVal)
	}
	return nil
}

func readLeaf(ldb *lvldb.LevelDB) (string, error) {
	leafBlockKey := []byte("leaf")
	val, err := getValue(ldb, leafBlockKey)
	return val, err
}

func updateBestQC(ldb *lvldb.LevelDB, hexVal string, dryRun bool) error {
	key := []byte("best-qc")
	v, _ := hex.DecodeString(hexVal)
	if dryRun {
		ldb.Put(key, v)
		fmt.Println("best-qc updated:", hexVal)
	} else {
		fmt.Println("best-qc will be:", hexVal)
	}
	return nil
}

func readBestQC(ldb *lvldb.LevelDB) (string, error) {
	key := []byte("best-qc")
	val, err := getValue(ldb, key)
	return val, err
}

func updateBest(ldb *lvldb.LevelDB, hexVal string, dryRun bool) error {
	key := []byte("best")
	v, _ := hex.DecodeString(hexVal)
	if dryRun {
		ldb.Put(key, v)
		fmt.Println("best updated:", hexVal)
	} else {
		fmt.Println("best will be:", hexVal)
	}
	return nil
}

func readBest(ldb *lvldb.LevelDB) (string, error) {
	key := []byte("best")
	val, err := getValue(ldb, key)
	return val, err
}
