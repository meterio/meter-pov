// Copyright (c) 2020 The Meter.io developers

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

	"encoding/hex"

	"github.com/dfinlab/meter/api/doc"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/crypto"
	tty "github.com/mattn/go-tty"
)

const paraString = "7479706520610a7120393838353834383131343738353339323431393933323931313633343633383233393237323539333630313734353437303331303937363333343133333530303632303839383839373036333235323836313831303431393231303532353631343638393937373833313833373239383735313336373034373032383734373731313033383234383836323436333937353435373032373639310a682031333532383333373038363039373437363036393233353830313130323833353636323231393236323833343938313336333130333037363536313036373633383333353631353531343531363531393734363630363036363434333134333539303831373330323933320a72203733303735313136373131343539353138363134323832393030323835333733393531393935383631343830323433310a65787032203135390a65787031203133380a7369676e3120310a7369676e30202d310a"

const systemString = "2db8cb49c44a1c7ba19fdaf6947425a7c0191c710b64fd89cdc8b573881d98d814e377bb5a158c90a93e077b6ec1c3c92ae51f53fb22ef42d117b95f84c2dfec00"

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

func fromBase64Pub(pub string) (*ecdsa.PublicKey, error) {
	b, err := b64.StdEncoding.DecodeString(pub)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPubkey(b)
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
			_, err := io.Copy(ioutil.Discard, r.Body)
			if err != nil {
				fmt.Println("could not copy x-genesis-id, error:", err)
			}

			http.Error(w, "genesis id mismatch", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// middleware to set 'x-meter-ver' to response headers.
func handleXMeterVersion(h http.Handler) http.Handler {
	const headerKey = "x-meter-ver"
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

func writeOutKeys(system bls.System, path string) (bls.PublicKey, bls.PrivateKey, error) {
	pubKey, privKey, err := bls.GenKeys(system)
	if err != nil {
		fmt.Println("GenKeys failed")
		return pubKey, privKey, err
	}

	pubBytes := system.PubKeyToBytes(pubKey)
	privBytes := system.PrivKeyToBytes(privKey)

	isolator := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	content := make([]byte, 0)
	content = append(content, pubBytes...)
	content = append(content, isolator...)
	content = append(content, privBytes...)

	err = ioutil.WriteFile(path, []byte(hex.EncodeToString(content)), 0644)
	if err != nil {
		fmt.Println("could not write out keys, error:", err)
	}

	return pubKey, privKey, nil
}

func getBlsSystem() (*bls.System, error) {
	paraBytes, err := hex.DecodeString(paraString)
	if err != nil {
		return nil, err
	}

	params, err := bls.ParamsFromBytes(paraBytes)
	if err != nil {
		return nil, err
	}

	pairing := bls.GenPairing(params)
	systemBytes, err := hex.DecodeString(systemString)
	if err != nil {
		return nil, err
	}

	system, err := bls.SystemFromBytes(pairing, systemBytes)
	if err != nil {
		return nil, err
	}

	return &system, nil
}
