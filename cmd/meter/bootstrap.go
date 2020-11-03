// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package main

import "github.com/ethereum/go-ethereum/p2p/discover"

var bootstrapNodes = []*discover.Node{
	// discover.MustParseNode("enode://bef967df827bfb246d837676beed52ce955f5376dda7dfb58f45b3f7a087c819e1821be7a9c45d89503344d4c9e65e015340e74b0a28f84fbad842430f5719d6@192.168.1.193:55555"),
	// discover.MustParseNode("enode://3a20f197837c01316f2e4fd6b165c82e07336724d976d049f7314803afebfd738728b02c04bc4e12f29e4b6cc8e527f7f19b18c0dd446858f33ebd79ea310c76@34.222.111.82:55555"),
	// discover.MustParseNode("enode://61f90566cfb85a0291882ebb97f89a0d896b17ca93fc08acd4558bfbaa25ca662619bd980505bc14523dad4cba97ad757bb77a70d72ba07f648018960814eafd@54.184.235.97:55555"),
	// discover.MustParseNode("enode://6865f570268591be82d5ec3dbdea1a1833bd3fedfec21812c71f743a9521577af134de5e3ef913264321de9c084726c393e2f057b0408fdcd1d2ff144585bbad@119.28.214.38:55555"),
	// discover.MustParseNode("enode://da424cb07d67400cc7e782b3d4b04c2170bddf073d665008ce3d33c332940c01881857edfb420bf1f74492d1d58becc73d20c3ef2d55d48593fa245ca2bde7a3@118.25.71.238:55555"),
	// discover.MustParseNode("enode://1c8532a2c2c99be0bfbde9171122132b63a1f6e0faf4a4554e3488688da18e4502ef850c67ed2ac01f9b5792f0c4c50e41dfad0457b7ef29f1e7595f4242637b@140.143.201.56:55555"),
	// discover.MustParseNode("enode://797fdd968592ca3b59a143f1aa2f152913499d4bb469f2bd5b62dfb1257707b4cb0686563fe144ee2088b1cc4f174bd72df51dbeb7ec1c5b6a8d8599c756f38b@107.150.112.22:55555"),
	// discover.MustParseNode("enode://a8a83b4faac13f0a05ecd383d661a85e15e2a93fb41c4b5d00976d0bb8e35aab58a6303fe6b437124888da45017b94df8ce72f6a8bb5bcfdc7bd8df51698ad01@106.75.226.133:55555"),
}

var KnownNodes = []*discover.Node{
	// Nodes for Yang
	//discover.MustParseNode("enode://a00f8a399e25e72d92534236d7683b9c4c23582045c7d081735e4badf09d0775797819791502893df901c6b9dd26281f31bf6850fdfcebb48ea37cdbf63a95ae@10.1.10.50:11235"),
	//discover.MustParseNode("enode://719d706c31723dca3cc5eb9c325b9bf842c4301ec3cc0086beb7823ecf428034b6a6c4f93efd1958a54746ee3d3312c3c26253c88b11c2cc6b08683a4e597190@10.1.10.9:11235"),

	// Nodes for Simon
	// discover.MustParseNode("enode://615dea026b3dac00b6b9935a412bed586ea4e5ce8ac9c2dc45db40c4cdeee2fd4db24c5af897dc1b6516429a33f3e7840ea38feec637fd6d405e4c50776b5739@10.1.10.12:11235")
	// discover.MustParseNode("enode://4d06ba9da6a7121931113be6546f847a4ee24d23a9ba0c2126995d5431a07d0470ec4dbc6144903e6d149d45cacc034e7edb3e154622429f41b873befc7d8caa@10.1.10.183:11235")
}
