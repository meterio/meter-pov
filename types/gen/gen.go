// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package gen

//go:generate rm -rf ./compiled/
//go:generate solc --optimize-runs 200 --overwrite --bin-runtime --bin --abi -o ./compiled scriptengine.sol
//go:generate go-bindata -nometadata -ignore=_ -pkg gen -o bindata.go compiled/
