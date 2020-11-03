// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package common
import (
    "fmt"
)

// Fingerprint returns the first 6 bytes of a byte slice.
// If the slice is less than 6 bytes, the fingerprint
// contains trailing zeroes.
func Fingerprint(slice []byte) []byte {
	fingerprint := make([]byte, 6)
	copy(fingerprint, slice)
	return fingerprint
}

// byte32 to byte slice
func Byte32ToByteSlice(b [][32]byte) []byte {
    s := []byte{}
    for _, h := range b {
        s = append(s, h[:]...)
    }
    return s
}

//byte slice to hash slice
func ByteSliceToByte32(b []byte) [][32]byte {
    h := make([][32]byte, len(b)/32)
    if len(b)%32 != 0 {
        fmt.Println("input error ")
        return h
    }

    t := [32]byte{}
    for i := 0; i < (len(b) / 32); i++ {
        copy(t[:], b[i*32:(i+1)*32])
        h[i] = t
    }
    return h
}

