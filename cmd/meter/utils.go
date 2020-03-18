// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import (
	"bytes"
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
	sha256 "crypto/sha256"

	"strings"

	"encoding/hex"

	"github.com/dfinlab/meter/api/doc"
	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/dfinlab/meter/consensus"
	tty "github.com/mattn/go-tty"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
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

func updatePublicKey(path string, pubKey *ecdsa.PublicKey, blsKeyStr string) error {
	b := b64.StdEncoding.EncodeToString(crypto.FromECDSAPub(pubKey))
	b = b + ":::" + blsKeyStr
	return ioutil.WriteFile(path, []byte(b+"\n"), 0600)
}

func fromBase64Pub(pub string) (*ecdsa.PublicKey, error) {
	b, err := b64.StdEncoding.DecodeString(pub)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPubkey(b)
}

// Save public key with BASE64 encoding
func loadOrUpdatePublicKey(path string, privKey *ecdsa.PrivateKey, newPubKey *ecdsa.PublicKey, blsKeyStr string) (*ecdsa.PublicKey, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return newPubKey, updatePublicKey(path, newPubKey, blsKeyStr)
	}

	s := strings.TrimSuffix(string(b), "\n")
	split := strings.Split(s, ":::")

	key, err := fromBase64Pub(split[0])
	if err != nil {
		return newPubKey, updatePublicKey(path, newPubKey, blsKeyStr)
	}

	if !verifyPublicKey(privKey, key) {
		return newPubKey, updatePublicKey(path, newPubKey, blsKeyStr)
	}

	if  len(split) <= 1 || split[1] != blsKeyStr {
		fmt.Println("not the same bls key")
		return key, updatePublicKey(path, key, blsKeyStr)
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

func writeOutKeys(system bls.System, path string) (bls.PublicKey, bls.PrivateKey, error){
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

	ioutil.WriteFile(path, []byte(hex.EncodeToString(content)), 0644)
	return pubKey, privKey, nil
}

func loadOrGenerateBlsCommon(path string) (*consensus.BlsCommon) {
	paraBytes, err := hex.DecodeString(paraString)
	if err != nil {
		fmt.Println("decode paraString error:", err)
		return nil
	}
	params, err :=  bls.ParamsFromBytes(paraBytes)
	if err != nil {
		fmt.Println("params from bytes error:", err)
		return nil
	}
	pairing := bls.GenPairing(params)

	systemBytes, err := hex.DecodeString(systemString)
	if err != nil {
		fmt.Println("decode system string error:", err)
		return nil
	}

	system, err := bls.SystemFromBytes(pairing, systemBytes)
	if err != nil {
		fmt.Println("system from bytes error:", err)
		return nil
	}

	readBytes, err := ioutil.ReadFile(path);
	if err == nil {
		keyBytes, err := hex.DecodeString(string(readBytes))
		if err == nil {
			isolator := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
			split := bytes.Split(keyBytes, isolator)
			pubKeyBytes := split[0]
			privKeyBytes := split[1]

			pubKey, err := system.PubKeyFromBytes(pubKeyBytes)

			if err == nil {
				privKey, err := system.PrivKeyFromBytes(privKeyBytes)
				if err == nil {
					/* verify loaded keys */
					signMsg := string("This is a test")
					msgHash := sha256.Sum256([]byte(signMsg))
					sig := bls.Sign(msgHash, privKey)
					valid := bls.Verify(sig, msgHash, pubKey)
					if valid == true {
						fmt.Println("loaded keys verify ok")
						return consensus.NewBlsCommonFromParams(pubKey, privKey, system, params, pairing)
					} else {
						fmt.Println("loaded key verify err")
					}
				} else {
					fmt.Println("priv key from bytes err")
				}
			} else {
				fmt.Println("pub key from bytes err")
			}
		} else {
			fmt.Println("decode keys str error ", err)
		}
        } else {
		fmt.Println("failed to read consensus key file ", err)
	}
	pubKey, privKey, err := writeOutKeys(system, path)
	if err != nil {
		fmt.Println("failed to write keys ", err)
		return nil
	}
	return consensus.NewBlsCommonFromParams(pubKey, privKey, system, params, pairing)
}
