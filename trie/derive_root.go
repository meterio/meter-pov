package trie

import (
	"bytes"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/dfinlab/meter/meter"
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
		rlp.Encode(keybuf, uint(i))
		trie.Update(keybuf.Bytes(), list.GetRlp(i))
	}
	return trie.Hash()
}
