// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package script_test

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/dfinlab/meter/reward"
	"github.com/dfinlab/meter/script"
	"github.com/dfinlab/meter/script/staking"
)

func TestReward(t *testing.T) {
	r := reward.RewardInfo{}
	fmt.Println("YYYYYYYYYYYYYY", r)
}

/*
Execute this test with
cd /tmp/meter-build-xxxxx/src/github.com/dfinlab/meter/script/staking
GOPATH=/tmp/meter-build-xxxx/:$GOPATH go test
*/

/*
func TestGob1(t *testing.T) {
	fmt.Println("Test Gob 1")
	raw := "0dffb1020102ffb20001ffb00000ffb1ffaf030102ffb000010b010941756374696f6e494401ff8800010b5374617274486569676874010600010a537461727445706f63680106000109456e644865696768740106000108456e6445706f63680106000108526c73644d54524701ff8600010952737664507269636501ff8600010a43726561746554696d650106000107526376644d545201ff8600010b41637475616c507269636501ff8600010c4c6566746f7665724d54524701ff8600000017ff87010101074279746573333201ff88000106014000000aff85050102ff8e000000fe03d4ffb2000801207fff9e08ffdaffab355e0fff881570fff74463ffea1eff8e276affb21f7dff9dffbc3540ffa47a0cff99ff99ffe00101010101fe0c850118010b020373c0f566173660000001090206f05b59d3b2000001fc5e9a2dc101010201090206f05b59d3b20000010b020373c0f5661736600000000120ff9b3d1d1d7fffd3ffaeff83ffc3ffc52cffe603ffa028517732247157ffdeffe2ff97ffa6ff9fff9bffd72bffe9ffdaffa001fe0c86011901fe11840130010b020373b7bce81846c0000001090206f05b59d3b2000001fc5e9a3a2001010201090206f05b59d3b20000010b020373b7bce81846c00000000120ff91ffe132ffbbff96ffc726ffdd12ffb5ffb76dffb71afff811ffaa6f5bff84fff21203fff0ffdb6803ffcd1e5f0bffde01fe1185013101fe16a00148010b020373ae8482ba2d40000001090206f05b59d3b2000001fc5e9a46bb01010201090206f05b59d3b20000010b020373ae8482ba2d400000000120ffa203ffb1ffc3ffc500ffe1ffdaffdf27ffd1ffeeffd9ff9130ffeb031afffe05ff99ffdaffaf02ffefff91ff95322075fff2ffac01fe16a1014901fe1c5d0160010b020373a54c35fca860000001090206f05b59d3b2000001fc5e9a54b601010201090206f05b59d3b20000010b020373a54c35fca860000000012045ffc3ffe6ffadff84ffc0fffeff8a77141669ffd6ffba37ff89ffa87548ffcf22435cffcd70ffcb2674ff9dffd1ff8cffd901fe1c5e016101fe21160178010b0203739c1401df7620000001090206f05b59d3b2000001fc5e9a608501010201090206f05b59d3b20000010b0203739c1401df76200000000120ffcbffd3ffad3fffb473ff93ffebffacffdf39ff8544ffe60fffc3ffff09ffe3ffe223fff0fff5ffc5ffdc6825ffbcffbeff903cff8201fe2117017901fe22c801ff90010b02037392dbe66254c0000001090206f05b59d3b2000001fc5e9a706401010201090206f05b59d3b20000010b02037392dbe66254c00000000120fff15c027eff8043ff88ffae5c7534ffbffff7ff8804ff99453039ffca3effe455ff901957ffd8ffa6ffea49ffb7ff8501fe22c901ff9101fe247401ffa8010b02037389a3e38502e0000001090206f05b59d3b2000001fc5e9a7f8101010201090206f05b59d3b20000010b02037389a3e38502e00000000120ffc650ff80ffda3c7fffa52746ffe4fffafff87e6b71ff9cffb8703801ffdfffbdffa36f03ffe4ffb10f1cffc2201f01fe247501ffa901fe25eb01ffc0010b020373806bf9473e20000001090206f05b59d3b2000001fc5e9a8dea01010201090206f05b59d3b20000010b020373806bf9473e20000000"
	bs, err := hex.DecodeString(raw)
	if err != nil {
		fmt.Println("ERROR:", err)
	}
	de := gob.NewDecoder(bytes.NewReader(bs))
	s := make([]*auction.AuctionSummary, 0)
	err = de.Decode(&s)
	if err != nil {
		fmt.Println("ERROR:", err)
	}
	sl := auction.NewAuctionSummaryList(s)
	fmt.Println(sl.String())
	for i, ss := range s {
		b := bytes.NewBuffer([]byte{})
		en := gob.NewEncoder(b)
		en.Encode(ss)
		fmt.Println("Summary #", i+1, hex.EncodeToString(b.Bytes()))
	}

		buf := bytes.NewBuffer([]byte{})
		en := gob.NewEncoder(buf)
		en.Encode(s)
		fmt.Println(hex.EncodeToString(buf.Bytes()))
		if bytes.Compare(buf.Bytes(), bs) != 0 {
			fmt.Println("BYTES DOESN'T MATCH!!!!! ")
		}
		de2 := gob.NewDecoder(bytes.NewReader(buf.Bytes()))
		s2 := make([]*auction.AuctionSummary, 0)
		de2.Decode(&s2)
		buf2 := bytes.NewBuffer([]byte{})
		en2 := gob.NewEncoder(buf2)
		en2.Encode(s2)
		if len(s) != len(s2) {
			fmt.Println("LLLLLLLLLLLLLLLLLLLLLLLLLLL")
		}
		for i, as := range s {
			as2 := s2[i]
			if as.ToString() != as2.ToString() {
				fmt.Println("XXXXXXXXXXXXXXXXX:", i)
			}
		}

		fmt.Println(hex.EncodeToString(buf2.Bytes()))
		if bytes.Compare(buf2.Bytes(), buf.Bytes()) != 0 {
			fmt.Println("_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
		}
}

func TestGob2(t *testing.T) {
	fmt.Println("Test Gob 2")
	raw := "0dffab020102ffac0001ffaa0000ffb1ffa9030102ffaa00010b010941756374696f6e494401ff8800010b5374617274486569676874010600010a537461727445706f63680106000109456e644865696768740106000108456e6445706f63680106000108526c73644d54524701ff8600010952737664507269636501ff8600010a43726561746554696d650106000107526376644d545201ff8600010b41637475616c507269636501ff8600010c4c6566746f7665724d54524701ff8600000017ff87010101074279746573333201ff88000106014000000aff85050102ff8e000000fe03d4ffac000801207fff9e08ffdaffab355e0fff881570fff74463ffea1eff8e276affb21f7dff9dffbc3540ffa47a0cff99ff99ffe00101010101fe0c850118010b020373c0f566173660000001090206f05b59d3b2000001fc5e9a2dc101010201090206f05b59d3b20000010b020373c0f5661736600000000120ff9b3d1d1d7fffd3ffaeff83ffc3ffc52cffe603ffa028517732247157ffdeffe2ff97ffa6ff9fff9bffd72bffe9ffdaffa001fe0c86011901fe11840130010b020373b7bce81846c0000001090206f05b59d3b2000001fc5e9a3a2001010201090206f05b59d3b20000010b020373b7bce81846c00000000120ff91ffe132ffbbff96ffc726ffdd12ffb5ffb76dffb71afff811ffaa6f5bff84fff21203fff0ffdb6803ffcd1e5f0bffde01fe1185013101fe16a00148010b020373ae8482ba2d40000001090206f05b59d3b2000001fc5e9a46bb01010201090206f05b59d3b20000010b020373ae8482ba2d400000000120ffa203ffb1ffc3ffc500ffe1ffdaffdf27ffd1ffeeffd9ff9130ffeb031afffe05ff99ffdaffaf02ffefff91ff95322075fff2ffac01fe16a1014901fe1c5d0160010b020373a54c35fca860000001090206f05b59d3b2000001fc5e9a54b601010201090206f05b59d3b20000010b020373a54c35fca860000000012045ffc3ffe6ffadff84ffc0fffeff8a77141669ffd6ffba37ff89ffa87548ffcf22435cffcd70ffcb2674ff9dffd1ff8cffd901fe1c5e016101fe21160178010b0203739c1401df7620000001090206f05b59d3b2000001fc5e9a608501010201090206f05b59d3b20000010b0203739c1401df76200000000120ffcbffd3ffad3fffb473ff93ffebffacffdf39ff8544ffe60fffc3ffff09ffe3ffe223fff0fff5ffc5ffdc6825ffbcffbeff903cff8201fe2117017901fe22c801ff90010b02037392dbe66254c0000001090206f05b59d3b2000001fc5e9a706401010201090206f05b59d3b20000010b02037392dbe66254c00000000120fff15c027eff8043ff88ffae5c7534ffbffff7ff8804ff99453039ffca3effe455ff901957ffd8ffa6ffea49ffb7ff8501fe22c901ff9101fe247401ffa8010b02037389a3e38502e0000001090206f05b59d3b2000001fc5e9a7f8101010201090206f05b59d3b20000010b02037389a3e38502e00000000120ffc650ff80ffda3c7fffa52746ffe4fffafff87e6b71ff9cffb8703801ffdfffbdffa36f03ffe4ffb10f1cffc2201f01fe247501ffa901fe25eb01ffc0010b020373806bf9473e20000001090206f05b59d3b2000001fc5e9a8dea01010201090206f05b59d3b20000010b020373806bf9473e20000000"
	bs, err := hex.DecodeString(raw)
	if err != nil {
		fmt.Println("ERROR:", err)
	}
	de := gob.NewDecoder(bytes.NewReader(bs))
	s := make([]*auction.AuctionSummary, 0)
	err = de.Decode(&s)
	if err != nil {
		fmt.Println("ERROR:", err)
	}
	sl := auction.NewAuctionSummaryList(s)
	fmt.Println(sl.String())

	for i, ss := range s {
		b := bytes.NewBuffer([]byte{})
		en := gob.NewEncoder(b)
		en.Encode(ss)
		fmt.Println("Summary #", i+1, hex.EncodeToString(b.Bytes()))
	}

		buf := bytes.NewBuffer([]byte{})
		en := gob.NewEncoder(buf)
		en.Encode(s)
		fmt.Println(hex.EncodeToString(buf.Bytes()))
		if bytes.Compare(buf.Bytes(), bs) != 0 {
			fmt.Println("BYTES DOESN'T MATCH!!!!! ")
		}
}

type unit struct {
	a uint64
	b uint64
}

type prime struct {
	a *big.Int
	b *big.Int
}

func TestBasic(t *testing.T) {
	u := []*unit{&unit{2233, 4455}, &unit{5566, 7788}, &unit{9900, 1100}}
	buf := bytes.NewBuffer([]byte{})
	en := gob.NewEncoder(buf)
	en.Encode(u)
	fmt.Println(hex.EncodeToString(buf.Bytes()))

	u2 := make([]*unit, 0)
	de := gob.NewDecoder(bytes.NewReader(buf.Bytes()))
	de.Decode(&u2)
	buf2 := bytes.NewBuffer([]byte{})
	en2 := gob.NewEncoder(buf2)
	en2.Encode(u2)
	fmt.Println(hex.EncodeToString(buf2.Bytes()))
	if bytes.Compare(buf.Bytes(), buf2.Bytes()) != 0 {
		fmt.Println("------BASIC -------------- BYTES NOT MATCH!!!!!k")
	}
}

func TestPrime(t *testing.T) {
	u := []*prime{
		&prime{big.NewInt(1122), big.NewInt(4455)},
		&prime{big.NewInt(5566), big.NewInt(7788)},
		&prime{big.NewInt(9900000000), big.NewInt(11000000)},
	}
	buf := bytes.NewBuffer([]byte{})
	en := gob.NewEncoder(buf)
	en.Encode(u)
	fmt.Println(hex.EncodeToString(buf.Bytes()))
	for _, x := range u {
		fmt.Println(x)
	}

	u2 := make([]*prime, 0)
	de := gob.NewDecoder(bytes.NewReader(buf.Bytes()))
	de.Decode(&u2)
	buf2 := bytes.NewBuffer([]byte{})
	en2 := gob.NewEncoder(buf2)
	en2.Encode(u2)
	fmt.Println(hex.EncodeToString(buf2.Bytes()))
	if bytes.Compare(buf.Bytes(), buf2.Bytes()) != 0 {
		fmt.Println("------PRIME -------------- BYTES NOT MATCH!!!!!k")
	}
	u3 := make([]*prime, 0)
	de2 := gob.NewDecoder(bytes.NewReader(buf2.Bytes()))
	de2.Decode(&u3)
	for _, x := range u3 {
		fmt.Println(x)
	}
}

func TestEncode(t *testing.T) {

	rlsdMTRG := big.NewInt(0)
	rs, _ := hex.DecodeString("0373b7bce81846c00000")
	rlsdMTRG.SetBytes(rs)

	fmt.Println(rlsdMTRG.String())
	fmt.Println(rlsdMTRG.Uint64())
	s := auction.AuctionSummary{
		AuctionID:    meter.MustParseBytes32("0x7f9e08daab355e0f881570f74463ea1e8e276ab21f7d9dbc3540a47a0c9999e0"),
		StartHeight:  1,
		StartEpoch:   1,
		EndHeight:    3205,
		EndEpoch:     24,
		RlsdMTRG:     rlsdMTRG,
		RsvdPrice:    big.NewInt(500000000000000000),
		CreateTime:   1587162561,
		RcvdMTR:      big.NewInt(0),
		ActualPrice:  big.NewInt(500000000000000000),
		LeftoverMTRG: rlsdMTRG,
	}
	buf := bytes.NewBuffer([]byte{})
	en := gob.NewEncoder(buf)
	en.Encode(&s)
	fmt.Println("SUMMARY HEX:", hex.EncodeToString(buf.Bytes()))
}

func TestSummary1(t *testing.T) {
	raw1 := "ffb1ffaf030102ffb000010b010941756374696f6e494401ff8800010b5374617274486569676874010600010a537461727445706f63680106000109456e644865696768740106000108456e6445706f63680106000108526c73644d54524701ff8600010952737664507269636501ff8600010a43726561746554696d650106000107526376644d545201ff8600010b41637475616c507269636501ff8600010c4c6566746f7665724d54524701ff8600000017ff87010101074279746573333201ff88000106014000000aff85050102ff8e00000076ffb001207fff9e08ffdaffab355e0fff881570fff74463ffea1eff8e276affb21f7dff9dffbc3540ffa47a0cff99ff99ffe00101010101fe0c850118010b020373c0f566173660000001090206f05b59d3b2000001fc5e9a2dc101010201090206f05b59d3b20000010b020373c0f566173660000000"
	raw2 := "ffb1ffa9030102ffaa00010b010941756374696f6e494401ff8800010b5374617274486569676874010600010a537461727445706f63680106000109456e644865696768740106000108456e6445706f63680106000108526c73644d54524701ff8600010952737664507269636501ff8600010a43726561746554696d650106000107526376644d545201ff8600010b41637475616c507269636501ff8600010c4c6566746f7665724d54524701ff8600000017ff87010101074279746573333201ff88000106014000000aff85050102ff8e00000076ffaa01207fff9e08ffdaffab355e0fff881570fff74463ffea1eff8e276affb21f7dff9dffbc3540ffa47a0cff99ff99ffe00101010101fe0c850118010b020373c0f566173660000001090206f05b59d3b2000001fc5e9a2dc101010201090206f05b59d3b20000010b020373c0f566173660000000"

	b1, _ := hex.DecodeString(raw1)
	b2, _ := hex.DecodeString(raw2)
	// b3, _ := hex.DecodeString(raw3)

	de1 := gob.NewDecoder(bytes.NewReader(b1))
	de2 := gob.NewDecoder(bytes.NewReader(b2))
	// de3 := gob.NewDecoder(bytes.NewReader(b3))
	s1 := &auction.AuctionSummary{}
	s2 := &auction.AuctionSummary{}
	// s3 := &auction.AuctionSummary{}
	de1.Decode(s1)
	de2.Decode(s2)
	// de3.Decode(s3)

	out1 := bytes.NewBuffer([]byte{})
	out2 := bytes.NewBuffer([]byte{})
	en1 := gob.NewEncoder(out1)
	en2 := gob.NewEncoder(out2)
	en1.Encode(s1)
	en2.Encode(s2)

	if bytes.Compare(b1, out1.Bytes()) != 0 {
		fmt.Println("1 can not be back")
		fmt.Println("HEX: ", hex.EncodeToString(out1.Bytes()))
	}
	if bytes.Compare(b2, out2.Bytes()) != 0 {
		fmt.Println("2 can not be back")
		fmt.Println("HEX: ", hex.EncodeToString(out2.Bytes()))
	}

	// if bytes.Compare(s1.AuctionID.Bytes(), s2.AuctionID.Bytes()) != 0 || bytes.Compare(s2.AuctionID.Bytes(), s3.AuctionID.Bytes()) != 0 || bytes.Compare(s1.AuctionID.Bytes(), s3.AuctionID.Bytes()) != 0 {
	// 	fmt.Println("AUCTION ID mismatch")
	// }
	// if bytes.Compare(s1.RlsdMTRG.Bytes(), s2.RlsdMTRG.Bytes()) != 0 || bytes.Compare(s3.RlsdMTRG.Bytes(), s2.RlsdMTRG.Bytes()) != 0 || bytes.Compare(s1.RlsdMTRG.Bytes(), s3.RlsdMTRG.Bytes()) != 0 {
	// 	fmt.Println("RELEASED MTRG mismatch")
	// 	fmt.Println(hex.EncodeToString(s1.RlsdMTRG.Bytes()))
	// 	fmt.Println(hex.EncodeToString(s2.RlsdMTRG.Bytes()))
	// 	fmt.Println(hex.EncodeToString(s3.RlsdMTRG.Bytes()))
	// }
	// if bytes.Compare(s1.RsvdPrice.Bytes(), s2.RsvdPrice.Bytes()) != 0 || bytes.Compare(s3.RsvdPrice.Bytes(), s2.RsvdPrice.Bytes()) != 0 || bytes.Compare(s1.RsvdPrice.Bytes(), s3.RsvdPrice.Bytes()) != 0 {
	// 	fmt.Println("RESERVED PRICE mismatch")
	// }
	// if bytes.Compare(s1.RcvdMTR.Bytes(), s2.RcvdMTR.Bytes()) != 0 || bytes.Compare(s3.RcvdMTR.Bytes(), s2.RcvdMTR.Bytes()) != 0 || bytes.Compare(s1.RcvdMTR.Bytes(), s3.RcvdMTR.Bytes()) != 0 {
	// 	fmt.Println("RECV MTR mismatch")
	// }
	// if bytes.Compare(s1.ActualPrice.Bytes(), s2.ActualPrice.Bytes()) != 0 || bytes.Compare(s3.ActualPrice.Bytes(), s2.ActualPrice.Bytes()) != 0 || bytes.Compare(s1.ActualPrice.Bytes(), s3.ActualPrice.Bytes()) != 0 {
	// 	fmt.Println("Actual Price mismatch")
	// }
	// if bytes.Compare(s1.LeftoverMTRG.Bytes(), s2.LeftoverMTRG.Bytes()) != 0 || bytes.Compare(s3.LeftoverMTRG.Bytes(), s2.LeftoverMTRG.Bytes()) != 0 || bytes.Compare(s1.LeftoverMTRG.Bytes(), s3.LeftoverMTRG.Bytes()) != 0 {
	// 	fmt.Println("Leftover MTRG mismatch")
	// 	fmt.Println(hex.EncodeToString(s1.LeftoverMTRG.Bytes()))
	// 	fmt.Println(hex.EncodeToString(s2.LeftoverMTRG.Bytes()))
	// 	fmt.Println(hex.EncodeToString(s3.LeftoverMTRG.Bytes()))

	// }

}

func encode(input interface{}) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(input)
	return buf.Bytes(), err
}

func TestT(t *testing.T) {
	s := auction.AuctionSummary{
		AuctionID: meter.MustParseBytes32("0x7f9e08daab355e0f881570f74463ea1e8e276ab21f7d9dbc3540a47a0c9999e0"),
	}
	b, err := encode(s)
	fmt.Println("err:", err)
	fmt.Println("1 -- ", hex.EncodeToString(b))
}


*/
func encode(obj interface{}) string {
	buf := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(buf)
	encoder.Encode(obj)
	return hex.EncodeToString(buf.Bytes())
}

type TestStruct1 struct {
	Id   string
	Addr string
}

func (t1 TestStruct1) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	i, err := fmt.Fprintln(&b, t1.Id, t1.Addr)
	fmt.Println(fmt.Sprintf("Written %v bytes", i), "error: ", err)
	return b.Bytes(), nil
}

func (t1 TestStruct1) UnmarshalBinary(data []byte) error {
	// A simple encoding: plain text.
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &t1.Id, &t1.Addr)
	return err
}

type TestStruct2 struct {
	Obj  TestStruct1
	Name string
	Num  uint64
}

func (t2 TestStruct2) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	i, err := fmt.Fprintln(&b, t2.Obj.Id, t2.Obj.Addr, t2.Name, t2.Num)
	fmt.Println(fmt.Sprintf("Written %v bytes", i), "error: ", err)
	return b.Bytes(), nil
}

func (t2 TestStruct2) UnmarshalBinary(data []byte) error {
	// A simple encoding: plain text.
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &t2.Obj.Id, &t2.Obj.Addr, &t2.Name, &t2.Num)
	return err
}

func TestGob(t *testing.T) {
	// gob.Register(big.NewInt(0))

	// gob.Register(meter.MustParseAddress("0x0000000000000000000000000000000000000000"))
	// gob.Register(meter.MustParseBytes32("0x0000000000000000000000000000000000000000000000000000000000000000"))
	// gob.Register(auction.AuctionSummary{})
	// gob.Register(&auction.AuctionSummary{})
	// gob.Register(auction.AuctionSummaryList{})
	// gob.Register(&auction.AuctionSummaryList{})
	// gob.Register(auction.AuctionCB{})
	// gob.Register(&auction.AuctionCB{})
	// gob.Register(&auction.AuctionTx{})
	gob.Register("")
	gob.RegisterName("TestStruct1", TestStruct1{})
	gob.Register(TestStruct2{})

	var buf bytes.Buffer
	var encoder *gob.Encoder
	var decoder *gob.Decoder
	var err error
	/*
		s1 := &auction.AuctionSummary{
			AuctionID: meter.MustParseBytes32("0x7f9e08daab355e0f881570f74463ea1e8e276ab21f7d9dbc3540a47a0c9999e0"),
		}
		s2 := &auction.AuctionSummary{
			AuctionID: meter.MustParseBytes32("0xa203b1c3c500e1dadf27d1eed99130eb031afe0599daaf02ef9195322075f2ac"),
		}

		sl := auction.AuctionSummaryList{
			Summaries: []*auction.AuctionSummary{s1, s2},
		}

		cb := auction.AuctionCB{
			AuctionID: meter.MustParseBytes32("0xcbd3ad3fb47393ebacdf398544e60fc3ff09e3e223f0f5c5dc6825bcbe903c82"),
		}
	*/

	s1 := TestStruct1{"0x7f9e08daab355e0f881570f74463ea1e8e276ab21f7d9dbc3540a47a0c9999e0", "somewhere"}
	s2 := TestStruct2{s1, "test", uint64(1234)}

	buf = *bytes.NewBuffer([]byte{})
	encoder = gob.NewEncoder(&buf)
	err = encoder.Encode(s1)
	if err != nil {
		fmt.Println("ERROR ENCODE s2:", err)
	}
	s1Bytes := buf.Bytes()
	fmt.Println("s1 HEX:", hex.EncodeToString(s1Bytes))
	fmt.Println(len(s1Bytes))
	buf = *bytes.NewBuffer(nil)
	encoder.Encode(s1)
	s1KnownBytes := buf.Bytes()
	fmt.Println("S1 HEX 2nd:", hex.EncodeToString(s1KnownBytes))

	buf = *bytes.NewBuffer([]byte{})
	encoder = gob.NewEncoder(&buf)
	err = encoder.Encode(s2)
	if err != nil {
		fmt.Println("ERROR ENCODE s1:", err)
	}
	s2Bytes := buf.Bytes()
	fmt.Println("s2 HEX:", hex.EncodeToString(s2Bytes))
	fmt.Println(len(s2Bytes))

	t1 := TestStruct1{}
	decoder = gob.NewDecoder(bytes.NewReader(s1Bytes))
	err = decoder.Decode(&t1)
	if err != nil {
		fmt.Println("ERROR DECODE t1:", err)
	}
	fmt.Println("t1: ", t1.Addr, t1.Id)

	t2 := TestStruct2{}
	decoder = gob.NewDecoder(bytes.NewReader(s2Bytes))
	err = decoder.Decode(&t2)
	if err != nil {
		fmt.Println("ERROR DECODE t2:", err)
	}

	fmt.Println("t2: ", t2.Obj.Addr, t2.Obj.Id, t2.Name, t2.Num)

	t1known := TestStruct1{}
	decoder = gob.NewDecoder(bytes.NewReader(s1KnownBytes))
	err = decoder.Decode(&t1known)
	if err != nil {
		fmt.Println("ERROR DECODE t1:", err)
	}
	fmt.Println("t1known: ", t1.Addr, t1.Id)

}

func TestScriptDecode(t *testing.T) {
	data, err := hex.DecodeString("deadbeeff9013fc4808203e8b90137f901340380019405771b188f4b5edcb94bbb5a9e3b5f9b312cc0189405771b188f4b5edcb94bbb5a9e3b5f9b312cc018866e75746b6161b8b34247726d66477833696a6944763342324a694949576263684e61436d5638576a5245337033504d487349774a472b694633417a61512b42574f795339517051684c34526b4d686447514b68325433377130736139514b383d3a3a3a516357385541536d714c6f4e677a7370676c4a5977514c7a644477307253306756386a452f7070776d355859663161516d79746f6a78767246444b347536425742477a6c704b754f32636c4c6e57616354496a694a77453d8d3131362e3230332e37312e36308221dea00000000000000000000000000000000000000000000000000000000000000000891043561a882930000001845ed1725b871c211be0f7eb7e80")

	if err != nil {
		fmt.Println("decode string failed", err)
		t.Fail()
		return
	}

	if bytes.Compare(data[:len(script.ScriptPattern)], script.ScriptPattern[:]) != 0 {
		fmt.Printf("Pattern mismatch, pattern = %v", hex.EncodeToString(data[:len(script.ScriptPattern)]))
		t.Fail()
		return
	}

	s, err := script.ScriptDecodeFromBytes(data[len(script.ScriptPattern):])
	if err != nil {
		fmt.Println("Decode script message failed", err)
		t.Fail()
		return
	}
	fmt.Println("Script header", s.Header.ToString())

	sb, err := staking.StakingDecodeFromBytes(s.Payload)
	if err != nil {
		fmt.Println("Decode script message failed", "error", err)
		t.Fail()
		return
	}
	fmt.Println("staking Body", sb.ToString())
}
