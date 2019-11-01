// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	b64 "encoding/base64"
	"strings"

	// "encoding/hex"

	"github.com/dfinlab/meter/api/doc"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/crypto"
	tty "github.com/mattn/go-tty"
)

func fatal(args ...interface{}) {
	var w io.Writer
	if runtime.GOOS == "windows" {
		// The SameFile check below doesn't work on Windows.
		// stdout is unlikely to get redirected though, so just print there.
		w = os.Stdout
	} else {
		outf, _ := os.Stdout.Stat()
		errf, _ := os.Stderr.Stat()
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

func loadOrGeneratePrivateKey(path string) (*ecdsa.PrivateKey, error) {
	key, err := crypto.LoadECDSA(path)
	if err == nil {
		return key, nil
	}

	if !os.IsNotExist(err) {
		return nil, err
	}

	key, err = crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	if err := crypto.SaveECDSA(path, key); err != nil {
		return nil, err
	}
	return key, nil
}

func verifyPublicKey(privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) bool {
	hash := []byte("testing")
	r, s, err := ecdsa.Sign(strings.NewReader("test-plain-text-some-thing"), privKey, hash)
	if err != nil {
		fmt.Println("Error during sign: ", err)
		return false
	}
	return ecdsa.Verify(pubKey, hash, r, s)
}

func updatePublicKey(path string, pubKey *ecdsa.PublicKey) error {
	b := b64.StdEncoding.EncodeToString(crypto.FromECDSAPub(pubKey))
	return ioutil.WriteFile(path, []byte(b), 0600)
}

func fromBase64Pub(pub string) (*ecdsa.PublicKey, error) {
	b, err := b64.StdEncoding.DecodeString(pub)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPubkey(b)
}

// Save public key with BASE64 encoding
func loadOrUpdatePublicKey(path string, privKey *ecdsa.PrivateKey, newPubKey *ecdsa.PublicKey) (*ecdsa.PublicKey, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return newPubKey, updatePublicKey(path, newPubKey)
	}

	key, err := fromBase64Pub(string(b))
	if err != nil {
		return newPubKey, updatePublicKey(path, newPubKey)
	}

	if !verifyPublicKey(privKey, key) {
		return newPubKey, updatePublicKey(path, newPubKey)
	}

	// k := hex.EncodeToString(crypto.FromECDSAPub(pubKey))
	return key, err
}

// copy from go-ethereum
func defaultDataDir() string {
	// Try to place the data folder in the user's home dir
	if home := homeDir(); home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Application Support", "org.dfinlab.meter")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "org.dfinlab.meter")
		} else {
			return filepath.Join(home, ".org.dfinlab.meter")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
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

// middleware to limit request body size.
func requestBodyLimit(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 96*1000)
		h.ServeHTTP(w, r)
	})
}

// middleware to verify 'x-genesis-id' header in request, and set to response headers.
func handleXGenesisID(h http.Handler, genesisID meter.Bytes32) http.Handler {
	const headerKey = "x-genesis-id"
	expectedID := genesisID.String()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualID := r.Header.Get(headerKey)
		w.Header().Set(headerKey, expectedID)
		if actualID != "" && actualID != expectedID {
			io.Copy(ioutil.Discard, r.Body)
			http.Error(w, "genesis id mismatch", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// middleware to set 'x-thorest-ver' to response headers.
func handleXThorestVersion(h http.Handler) http.Handler {
	const headerKey = "x-thorest-ver"
	ver := doc.Version()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerKey, ver)
		h.ServeHTTP(w, r)
	})
}

// middleware for http request timeout.
func handleAPITimeout(h http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		r = r.WithContext(ctx)
		h.ServeHTTP(w, r)
	})
}

func readPasswordFromNewTTY(prompt string) (string, error) {
	t, err := tty.Open()
	if err != nil {
		return "", err
	}
	defer t.Close()
	fmt.Fprint(t.Output(), prompt)
	pass, err := t.ReadPasswordNoEcho()
	if err != nil {
		return "", err
	}
	return pass, err
}
