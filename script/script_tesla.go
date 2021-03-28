// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script

import (
	"fmt"
	"math/big"

	"github.com/dfinlab/meter/meter"
	"github.com/dfinlab/meter/script/staking"
	"github.com/dfinlab/meter/state"
)

func EnterTeslaForkInit() {
	se := GetScriptGlobInst()
	if se == nil {
		panic("get script engine failed ... ")
	}

	se.StartTeslaForkModules()
	se.logger.Info("enabled script tesla modules ... ")
}

func EnforceTeslaFork1_1Corrections(state *state.State, ts uint64) {
	se := GetScriptGlobInst()
	if se == nil {
		panic("get script engine failed ... ")
	}

	mod, find := se.modReg.Find(STAKING_MODULE_ID)
	if find == false {
		err := fmt.Errorf("could not address module %v", STAKING_MODULE_ID)
		fmt.Println(err)
		return
	}

	stk := mod.modPtr.(*staking.Staking)
	if stk == nil {
		err := fmt.Errorf("could not address module %v", STAKING_MODULE_ID)
		fmt.Println(err)
		return
	}
	// fixed wrong data
	corrections := LoadStakeCorrections()
	for _, c := range corrections {
		if c.MeterGovAmount.Sign() == 0 {
			continue
		}

		stk.EnforceTeslaFor1_1Correction(c.BucketID, c.Addr, c.MeterGovAmount, state, ts)
	}
	fmt.Println("EnforceTeslaFork1_1Corrections done...")
}

var TeslaFork_1_1_Correction [][3]string = [][3]string{
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "78000"},                   // 9470826
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "78000"},                   // 9471355
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "78423"},                   // 9470777
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "78000"},                   // 9471156
	{"0x8bf74bfc7021e2dbd205a0d87388cf3f3c7521248ffdb4044a76a506c7fea5f0", "0x16fb7dc58954fc1fa65318b752fc91f2824115b6", "2000"},                    // 9474260
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "2000"},                    // 9474932
	{"0x285dfdd2dd03c5b064cb9b189fa9d9d2c5ae75dbf782e2bad2ce55aa6a9caa17", "0xa13e81b67938821e0df2d7057f0d1345a1ed7253", "12"},                      // 9477822
	{"0x8231c3fe911c22334628690ec47ecc1b0143b5402de32c444d37f1637bb9bc5c", "0xce0fb318f31afb6d616ec58f8fa9c1aae206f4a8", "129"},                     // 9478932
	{"0x14f4085a3171fc649e7cb397e86eaaf61b549974a56456874a621c1bf88f944e", "0xdf33800a566f993ea50c409a5d85b7170b60c0fd", "50"},                      // 9480763
	{"0xfc99c5d451adafdf87aa426e4bf902072c91f39b25ea4b123771cf1f12f76032", "0xe2aa22be6d1c042bbbbc96f9284ef48750627a4d", "20"},                      // 9509532
	{"0x3e0df027ebe98051f11e3b5684ed174e00024a83c757e994c856ef86c8faf796", "0xf2c4b0b9c08cd870ec11636d5e028424b6796b12", "395"},                     // 9505965
	{"0xfc99c5d451adafdf87aa426e4bf902072c91f39b25ea4b123771cf1f12f76032", "0xe2aa22be6d1c042bbbbc96f9284ef48750627a4d", "20"},                      // 9509476
	{"0xa21f2d103ac2d873d2ac023fe2431ea102ac921aab63c029e8be13a7c0d23130", "0x0f8684f6dc76617d6831b4546381eb6cfb1c559f", "13900"},                   // 9513458
	{"0x285dfdd2dd03c5b064cb9b189fa9d9d2c5ae75dbf782e2bad2ce55aa6a9caa17", "0xa13e81b67938821e0df2d7057f0d1345a1ed7253", "12"},                      // 9512022
	{"0xa21f2d103ac2d873d2ac023fe2431ea102ac921aab63c029e8be13a7c0d23130", "0x0f8684f6dc76617d6831b4546381eb6cfb1c559f", "13900"},                   // 9513342
	{"0x285dfdd2dd03c5b064cb9b189fa9d9d2c5ae75dbf782e2bad2ce55aa6a9caa17", "0xa13e81b67938821e0df2d7057f0d1345a1ed7253", "12"},                      // 9513888
	{"0xa21f2d103ac2d873d2ac023fe2431ea102ac921aab63c029e8be13a7c0d23130", "0x0f8684f6dc76617d6831b4546381eb6cfb1c559f", "100"},                     // 9520073
	{"0xdab0fbcd5d66bbd4fb974e82e173dbecde327e170c2281bcb58e8707e9736d71", "0x353fdd79dd9a6fbc70a59178d602ad1f020ea52f", "3000"},                    // 9518525
	{"0xfc99c5d451adafdf87aa426e4bf902072c91f39b25ea4b123771cf1f12f76032", "0xe2aa22be6d1c042bbbbc96f9284ef48750627a4d", "20"},                      // 9522116
	{"0x9994d436d8c8a9ac0729d4bbd04dea6d22abbe688fe29b6f8a53a390905fb9fc", "0x8cddd94f4ff868111b4fed54d149a791e91099fb", "88"},                      // 9524491
	{"0x9994d436d8c8a9ac0729d4bbd04dea6d22abbe688fe29b6f8a53a390905fb9fc", "0x8cddd94f4ff868111b4fed54d149a791e91099fb", "105"},                     // 9525046
	{"0x3d66e24ebfe0323633a94b0fa587892f14459338958dbdecd632bbbbd4a1f29d", "0xbf85ef4216340eb5cd3c57b550aae7a2712d48d2", "1"},                       // 9532181
	{"0x3d66e24ebfe0323633a94b0fa587892f14459338958dbdecd632bbbbd4a1f29d", "0xbf85ef4216340eb5cd3c57b550aae7a2712d48d2", "1000000000000000000"},     // 9538201
	{"0xea65a1fd67b511fefc4ed08e2fd52c02525617dca80b92895603fe95b5fe4cc8", "0x5bfef0997ce0ea62cb29fffb28ad2e187e51af26", "100"},                     // 9546298
	{"0x285dfdd2dd03c5b064cb9b189fa9d9d2c5ae75dbf782e2bad2ce55aa6a9caa17", "0xa13e81b67938821e0df2d7057f0d1345a1ed7253", "12"},                      // 9552323
	{"0xa21f2d103ac2d873d2ac023fe2431ea102ac921aab63c029e8be13a7c0d23130", "0x0f8684f6dc76617d6831b4546381eb6cfb1c559f", "13900"},                   // 9555247
	{"0x8f2d5c6e22dece3782bf59b0025bac2e4c083770ff5f56df1c924df2bf04c5f8", "0x848ec68f3221ee18a85e7117a1deb2f8c74e5795", "7784"},                    // 9558062
	{"0x8f2d5c6e22dece3782bf59b0025bac2e4c083770ff5f56df1c924df2bf04c5f8", "0x848ec68f3221ee18a85e7117a1deb2f8c74e5795", "7784"},                    // 9558092
	{"0xa21f2d103ac2d873d2ac023fe2431ea102ac921aab63c029e8be13a7c0d23130", "0x0f8684f6dc76617d6831b4546381eb6cfb1c559f", "1000"},                    // 9560340
	{"0xaa43da1be521ed86dee4a0aa8a697b5ad61561456b16616093c038d88bf0c2c2", "0xc885536c2b688859c13505f2f076bdfec204a880", "11"},                      // 9587661
	{"0xdfe33278c1fe11f943db9b8cc6367c377fd52b41fc1c5ebb28f00e07da8346dd", "0x71cd50ba682de704fa0010a2da238fddbc958697", "4000000000000000000"},     // 9577329
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "36000000000000000000"},    // 9576519
	{"0x562dc8d8a27da3b29ecec89b8cb4b4d18854f7f6c7c37b8dc4451607ca025bc5", "0xfb00059194cc3810e87adfa810e61bd61543a2c6", "977021483000000000000"},   // 9580697
	{"0x5703b719f6e5d57962f92da1e8ad9c45b59457c079f5b6ed2df76a915888569b", "0x1ce46b7bf47e144e3aa0203e5de1395e85fce087", "80000000000000000000000"}, // 9576603
	{"0x13a75bb68592621eb1ba66c48107efaee2f40a1da7b531e0a021bf70de7c798e", "0xf6959e47b94c82b93867fc876b4c99f37e496897", "6000000000000000000"},     // 9590447
	{"0xdfe33278c1fe11f943db9b8cc6367c377fd52b41fc1c5ebb28f00e07da8346dd", "0x71cd50ba682de704fa0010a2da238fddbc958697", "4000000000000000000"},     // 9590327
	{"0x04e26e966a6d4359c46e80d5446dd0a6237c89ccf316b8e9e5bb22f566aa49d4", "0x27cabef682992686a34e260574594aaf50aec80a", "278"},                     // 9594588
	{"0x04e26e966a6d4359c46e80d5446dd0a6237c89ccf316b8e9e5bb22f566aa49d4", "0x27cabef682992686a34e260574594aaf50aec80a", "278"},                     // 9594789
	{"0x46a14f30a002647d60441089019706ff7bdca31d1d70c5bdb1719be19e3b8cb3", "0x7db6ac6fa3c8aa2c6fb9bdcd4800e8baacdd2ee8", "1000000000000000000000"},  // 9612482
	{"0x46a14f30a002647d60441089019706ff7bdca31d1d70c5bdb1719be19e3b8cb3", "0x7db6ac6fa3c8aa2c6fb9bdcd4800e8baacdd2ee8", "1000000000000000000000"},  // 9612470
	{"0x338acce17c0f07c26d6c6355f7a512b80a0e2761783b7cde4a856bb710760881", "0xbd51fc2e0f304bf33eb532a4f98dcfafba0377b6", "60993377000000000000"},    // 9614489
	{"0x01a310324e1812cab7bf76521b64cf4c4cfb5a2b9a684066dd058447dd1aa330", "0x86f22c3eba0ce82f590e82a28d1838c891fc5cd6", "6000000000000000000"},     // 9616360
	{"0x7e11b8d844a216237b559bcabc18c82ac0c77225c68cc6aee72aee7583a240de", "0x81fc21bfccf416aa19672ef32cc5144598945664", "1000000000000000000"},     // 9653218
}

// Profile indicates the structure of a Profile
type StakingCorrection struct {
	BucketID       meter.Bytes32
	Addr           meter.Address
	MeterGovAmount *big.Int
}

func LoadStakeCorrections() []*StakingCorrection {
	corrections := make([]*StakingCorrection, 0, len(TeslaFork_1_1_Correction))
	for _, p := range TeslaFork_1_1_Correction {
		bktId := meter.MustParseBytes32(p[0])
		address := meter.MustParseAddress(p[1])

		mtrg, ok := new(big.Int).SetString(p[2], 10)
		if ok != true {
			fmt.Println("parse mtrg value failed")
			continue
		}

		pp := &StakingCorrection{
			BucketID:       bktId,
			Addr:           address,
			MeterGovAmount: mtrg,
		}
		fmt.Println("new stake correction created", "correction", bktId.String(), address.String(), mtrg.String())

		corrections = append(corrections, pp)
	}
	return corrections
}
