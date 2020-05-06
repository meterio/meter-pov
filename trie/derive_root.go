package trie

import (
	"bytes"
	"fmt"

	"github.com/dfinlab/meter/meter"
	"github.com/ethereum/go-ethereum/rlp"
)

// see "github.com/ethereum/go-ethereum/types/derive_sha.go"

type DerivableList interface {
	Len() int
	GetRlp(i int) []byte
}

func DeriveRoot(list DerivableList) meter.Bytes32 {
	keybuf := new(bytes.Buffer)
	trie := new(Trie)
	for i := 0; i < list.Len(); i++ {
		keybuf.Reset()
		if err := rlp.Encode(keybuf, uint(i)); err != nil {
			fmt.Printf("rlp Encode failed, error=%s\n", err.Error())
			continue
		}
		trie.Update(keybuf.Bytes(), list.GetRlp(i))
	}
	return trie.Hash()
}
