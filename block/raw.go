// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package block

// Raw allows to partially decode components of a block.
type Raw []byte

// DecodeHeader decode only the header.
func (r Raw) DecodeHeader() (*Header, error) {
	blk, err := r.DecodeBlockBody()
	if err != nil {
		return nil, err
	}
	return blk.Header(), nil

}

// DecodeBody decode only the body.
func (r Raw) DecodeBody() (*Body, error) {
	blk, err := r.DecodeBlockBody()
	if err != nil {
		return nil, err
	}
	return blk.Body(), nil
}

// XXX: Decode Evidence, CommitteeInfo, KBlockData

// DecodeBlockBody decode block header & tx part.
func (r Raw) DecodeBlockBody() (*Block, error) {
	return BlockDecodeFromBytes(r)
}
