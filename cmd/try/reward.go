package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"net"

	api_staking "github.com/dfinlab/meter/api/staking"
	bls "github.com/dfinlab/meter/crypto/multi_sig"
	"github.com/dfinlab/meter/reward"
	"github.com/dfinlab/meter/types"
)

/*
Name        []byte          `json:"name"`
	Address     meter.Address   `json:"address"`
	PubKey      ecdsa.PublicKey `json:"pub_key"`
	BlsPubKey   bls.PublicKey   `json:"bsl_pubkey"`
	VotingPower int64           `json:"voting_power"`
	NetAddr     NetAddress      `json:"network_addr"`
	Commission  uint64          `json:"commission"`
	DistList    []*Distributor  `json:"distibutor_list"`
*/
var (
	FakeECDSAPK = ecdsa.PublicKey{}
	FakeBlsPK   = bls.PublicKey{}
)

const delegateStr = `[{"name":"shoal-01","address":"0x658b6da6723979ec56fef1296f115477460a8797","pubKey":"BD8rZwjdir9hidtqxDK5rbbGrGSZ9BZ0R98jAtpxKdLJ0n3BbFeI05tbptWtvJ5Xmkm45cKwpGZqx39a0uMka90=:::f9uhSGcTXRBkOqpYuDkjw4kmcvmAELGau7MJVGgQwP+G63+vzwEU+WCu0e0NFBmJFpsT9ZQgIvapzpY1dviKFQA=","votingPower":"3011909512937595128533","ipAddr":"13.228.91.172","port":8670,"commission":100000000,"distributors":[{"address":"0x658b6da6723979ec56fef1296f115477460a8797","autobid":0,"shares":667984713},{"address":"0xc0f86b01d4b1725b8ab75af820224a5a0ac9b55e","autobid":0,"shares":332015286}]},{"name":"shoal-03","address":"0xe6c7317a261e0453a6793f0ce2466c508d3f2c04","pubKey":"BAYuQ+dcpHZP7fiBuKf0ddt9qqEbxek/5DFDgo897cR4HoVDXlrHtJvPdC6YCBM4UcSBq9+DBG99IsTEQnfSZaE=:::PKWq4VQqvX6+B+3GrvOnSELoPt2AhKfNozaY4iIz3t2DvoetKdD70rP0QwT86WncK1sUA70MJjOJ/+69ZAQ+AgE=","votingPower":"3011909081684424149334","ipAddr":"52.220.150.53","port":8670,"commission":100000000,"distributors":[{"address":"0xe6c7317a261e0453a6793f0ce2466c508d3f2c04","autobid":0,"shares":667984665},{"address":"0xf3dd5c55b96889369f714143f213403464a268a6","autobid":100,"shares":332015334}]},{"name":"shoal-05","address":"0x06423a48178615cf42f97b8e9b6f5351e351f652","pubKey":"BCXOoOuardRwYAHjsQ2LPJ7vuNWObLD951Cm8WWGZlKFHs/o+G9IHVxt8vAW3N3B6IhfHR17jIJSlus4YTVTwiQ=:::rWBD4IYkQKG/zJUy4rks3MZAGqbHMVpyoXISu2ljQJTEPFVdnlQAifSm8vtGMlgFxif9ljHXtUjJKWC0XTVBQAE=","votingPower":"3011908650431253170136","ipAddr":"54.254.11.210","port":8670,"commission":100000000,"distributors":[{"address":"0x06423a48178615cf42f97b8e9b6f5351e351f652","autobid":100,"shares":667984618},{"address":"0x03aa4784c850265fdc4260412c80d2551f329e0c","autobid":0,"shares":332015381}]},{"name":"shoal-07","address":"0xe4d638fa7c53181510f241766fa206206f058eb5","pubKey":"BNKAoqtHFbCEypdKQlSavh/v0PPAv3FWgMZGv9+ZuN9LKkE4enW1zUUyhrpuJ3YE68hMUI+2RSny6Yw3A49EPCg=:::Ur7lJlp9bC+uMAihvx68AhTSWnUnha1S6PkLMt6xgS6HRTMb3Bj6iGNAYGs1ufWXeqz9Su2K+93X96UXeXWC+wE=","votingPower":"3011908163368848299512","ipAddr":"44.241.85.51","port":8670,"commission":100000000,"distributors":[{"address":"0xe4d638fa7c53181510f241766fa206206f058eb5","autobid":100,"shares":667984564},{"address":"0xf30320537817d49ddb4f3f6878296e046de0a6fe","autobid":100,"shares":332015435}]},{"name":"shoal-02","address":"0x34bd9720f4d83db2c8d7de87ec38b7832301ca67","pubKey":"BHq/NmcbeOS/wEqZGGYOgm79/tLkZy004IFu4gjSt7jiD/fLrJsdKSHqe/oQqT5y78tt6H3zr016hvXte/Ntw70=:::bNpwfSjJbskU1czBj0p/2Y4s03jH8mx9nV+ahX55815HNRG03+nEjOLkh4WcNgUe2MQKVTw83mgNU8Ju79MF8gE=","votingPower":"2011909299847792997635","ipAddr":"18.136.189.43","port":8670,"commission":100000000,"distributors":[{"address":"0x34bd9720f4d83db2c8d7de87ec38b7832301ca67","autobid":0,"shares":1000000000}]},{"name":"shoal-04","address":"0x2b1003d406d30888be8bc645e542dec79607bad5","pubKey":"BBxoHjMZLC4akskv1uMr8lqpmfELemYB3P1piEoJukJlXXngMD4F/47jHTfsjPqAQeRfntIPW93FyBU7MZA2ouA=:::fwNuti4hN3GwdTPJaSSQGwU2OtWCx/8UdHeYtLEAn5WWiOjjhBKrjx2ccbVEQoniy2H/JL96kYPqpYwl712BiAA=","votingPower":"2011908863521055301034","ipAddr":"54.251.200.38","port":8670,"commission":100000000,"distributors":[{"address":"0x2b1003d406d30888be8bc645e542dec79607bad5","autobid":0,"shares":1000000000}]},{"name":"shoal-06","address":"0xe69bd369509203be30f863fd2f474e67fec8590c","pubKey":"BCyh9c6hHFb/ped4A2Y1XJTQja7fWrtvzyi6sBOZkoXNbd2UclKlI1U5lEw39iPZBpAwROI/9GSh2kQa6XttUT0=:::jPCfE+Ktn4JPMEcGGhskvIdN0j8bjSWMjbLPjZt7VWFbS21nWGdnQFEwhUKN83reMenfbXAgCYdqMW6Ly3azXwA=","votingPower":"2011908422120750887031","ipAddr":"54.254.108.229","port":8670,"commission":100000000,"distributors":[{"address":"0xe69bd369509203be30f863fd2f474e67fec8590c","autobid":0,"shares":1000000000}]},{"name":"shoal-08","address":"0x9bd0a74c7c2c51cf30246c87b875c8f51706a089","pubKey":"BJewT3v2FpdtKxtIS9+Fc5QIbBXT8sNTtp7XjZIonvVRBcGl9VbTaJjmyQQ24ARYnAud3oUyW8qSu+Gkq1xFen8=:::I77bzNnL7Xh5Sw8zvV0jnXfoEidJpvn9LdDnZ9b8mS0JgXoRBg5XOuY8WioFlv0BD3+Cwb6w3peNq3SVaHyxmgA=","votingPower":"2008769406392694063224","ipAddr":"52.24.8.47","port":8670,"commission":100000000,"distributors":[{"address":"0x9bd0a74c7c2c51cf30246c87b875c8f51706a089","autobid":0,"shares":1000000000}]},{"name":"shoal-09","address":"0x4affbb2515f15798bb0cc2e8198ebd0007015d67","pubKey":"BAM1lNIR004ScqCsmN3Cws5XYCpfbwserSPsWKpHFTGZ/Cd2PH6S/t5hfbBTnIC4sK43VUgdKAAv/4DgIwfqTLM=:::txF4WwPlDMrUseVf1F3X0IAnbQfqQ9YGN2QCHpOmtrI7L00hdGgWlAg0qw7P0lQLayDsVj4PS4US2zBafRrW4QA=","votingPower":"2008769167935058345315","ipAddr":"44.240.83.76","port":8670,"commission":100000000,"distributors":[{"address":"0x4affbb2515f15798bb0cc2e8198ebd0007015d67","autobid":0,"shares":1000000000}]},{"name":"shoal-10","address":"0x9ed1914f1a6759741a23c0ac1371243414ea612b","pubKey":"BIQSm1Pn4nCvr9dt9lNwL3jCjGwz9LaHw1HM4ccBFnz2VkK0Osy/zfRia5+mRKCSzPKBVZFbKf5SQJKOJmXR+Fk=:::Sc+cDxl8codYh2MTUtevsZS6ZwWX+gA+2n2zwpr+/JT4nsUjnSEgs5WQ+hHAysW4leybIICgeYqZ2V1XYqKAeAA=","votingPower":"2008768904109589040393","ipAddr":"44.228.229.107","port":8670,"commission":100000000,"distributors":[{"address":"0x9ed1914f1a6759741a23c0ac1371243414ea612b","autobid":0,"shares":1000000000}]},{"name":"shoal-11","address":"0xac4acda6d9e4471da5b1f3815c7e6952cd34cb1b","pubKey":"BL5Mlj7G3sprNERzdrVJnsf73W1vBlWXFam7ZL7fCJWFJHdRN+SAApRuMG9HDVGOV/9WNugsd2vQWlR1jYf8R3o=:::ZkhnePEX4ghmJFC0bRjJ38xDpznR1szI834AGXBoHpfpR4QhanLlgVztQ+Edig9ZuiN94EvIghZzdNW8NP01MgE=","votingPower":"2008768645357686452874","ipAddr":"44.236.203.192","port":8670,"commission":100000000,"distributors":[{"address":"0xac4acda6d9e4471da5b1f3815c7e6952cd34cb1b","autobid":0,"shares":1000000000}]}]`

func main() {
	apiDelegates := make([]*api_staking.Delegate, 0)
	json.Unmarshal([]byte(delegateStr), &apiDelegates)

	delegates := make([]*types.Delegate, 0)
	for _, d := range apiDelegates {
		dist := make([]*types.Distributor, 0)
		for _, dd := range d.DistList {
			dist = append(dist, &types.Distributor{
				Address: dd.Address,
				Shares:  dd.Shares,
				Autobid: dd.Autobid,
			})
		}
		vp := big.NewInt(0)
		vp.SetString(d.VotingPower, 10)
		vp.Div(vp, big.NewInt(1e12))
		ip := net.ParseIP(d.IPAddr)
		netAddr := types.NewNetAddressIPPort(ip, 8670)
		delegate := &types.Delegate{
			Name:        []byte(d.Name),
			Address:     d.Address,
			PubKey:      FakeECDSAPK,
			BlsPubKey:   FakeBlsPK,
			NetAddr:     *netAddr,
			VotingPower: vp.Int64(),
			Commission:  d.Commission,
			DistList:    dist,
		}
		delegates = append(delegates, delegate)
	}

	rewardMap, _ := reward.ComputeRewardMap(big.NewInt(400000000000000000), new(big.Int).Mul(big.NewInt(40638528332274), big.NewInt(100)), delegates)

	fmt.Println("**** Dist List")
	for _, d := range rewardMap.GetDistList() {
		fmt.Println(d.String())
	}
	fmt.Println("**** Autobid List")
	for _, a := range rewardMap.GetAutobidList() {
		fmt.Println(a.String())
	}
}
