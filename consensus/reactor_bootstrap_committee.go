package consensus

import (
	"strconv"

	"github.com/meterio/meter-pov/meter"
	"github.com/meterio/meter-pov/types"
)

var (
	m01ComboPubkey = "BEdvoJsZLZw2mmKPkZIpLQ3B9/L0warwsmmS1dNHSXfDP0vBx6JSYsmuWwj/3so5j/mn9OfTJMMEMO/LtRmSPOo=:::LWilIBiprQWu9CfOxUGME9l09994UozRan3iterG/2zqtUzE6nw2Pv8vpAFJSQTz4fbq2hd5dHrQIjWX574ftAA="
	m02ComboPubkey = "BEaaRrf4wJZhtGlgmiffJGxn0dcR7mIbS5qMcDBvbfDs/njjV4jdSvibcQdE6agklgenUnMTlA9/6Zd3it1HxWI=:::VB7HSRio7qM9ikpBsDZw4UKRVkxH1jSV6+BE9tq1i+LlIdH/iP3Gf7kwSBTgKxStlglSA9CMNYp4M+XeNXb1jAE="
	m03ComboPubkey = "BEZO/q3aKFSIXgIh50FKN8zlNMk8CKdiYeq+rNLnDIFTpM/LYqAguaDuNzxHTgFXiB/imQLj9ZlFQMK5j/Cj9PE=:::Ptux3rGIa1Isfw7UVy5+LkFzYI4FFV33ssqmO6qTD8ZeocZybNJwKPLIpgCurN2tjQDwGP1YKt1XZ4GLMl1OrwE="
	m04ComboPubkey = "BJmMSFBVnr0JSTo6MhJr5TWlVaKp88tIZhwsn83hnwwAa12WKJrgJLYgmXs3cudZoENDhqpVWLTo9mLogc5fZv4=:::Io+O9rtIHMRB+2b0KAnARYTe7pD+r3U3DrVS9dPEpHgKcLXSpdnkkx8pbfdBCz68s+stkWcajRCEHN0O1tbC4gE="
	m05ComboPubkey = "BBzIyiEmHlh+k9qbN/jQWmW7CfFzjmnNJPtSVCpPobH+0k/2zgtPic4UcHA7WiOrdWR6aAUML/eVvPH/p4LR2xo=:::PvuNxTb5AFHAQmZHhfJv3lS0i1dpIKNVuyUuaJSzFCtkG/Zhd9N0UAhe9I211GQL09ielD1MHAAvdvqUq/HxngA="
	m06ComboPubkey = "BJbdlkf5FYkKcBArsZoh2HiG9bO7e2Wpc5pPQ2vcsX5KTJr+gzUbCyvrThVXaGKWS2HUpXdqZ5kfPO2uAWR6X1I=:::BC0M7dmHdV3M0eSEQz3vli+n3FkS8mPPSz/Y7FA2r/ugqmqf12c4YLNVCdtLQAfmmfzQhXKQTqay+o3rNMf3AQE="
	m07ComboPubkey = "BBJ16RGbjlyunmWC8RcD5P7CGMH0dh7T3whhV3XNHaCA3kcbzGYeuc4E7T6W5xJO96ly3eEp5bhq9qdm/5Zpb4M=:::mFHRB+OK/l8ye2rwOZXO9Dnlu4rnPq6reaygdJ5dBiOkMYHSLlJ0A+bFhzabk+TB8NKGx617d5NZ69LS/J3qdgE="
	m08ComboPubkey = "BPO/wh3qI7oxiNZFkp7hNvL0NIPtQTzONkXq/oCIIYKmB3/FzRK7b2Z7kI/wnfeeyEDKbjfMicH1kbfxJRwMNx8=:::caxrbfsN8mxPYlNi9czhsA9TSOLt0Xp/npCmcfh/27fZcJgTYaQ4fTnPY4dEPReWJnvoDBzaowOZLRURaj1wbgA="
	m09ComboPubkey = "BJQkmBX6K24DPM920sK5CxRFEEzfGA0wIzNR/bTUpXBEp9AkNEArZJ/eMPBX7E/kcPvrv+VpESLH+ECs3wuUOdQ=:::OeyP//h+fig22Sh6uFL3ktVtOTyyPq2RAYbNmXCgRIRpcliJh2X6zOExIaZZbhv0SrDYPbSxEbhjW4mrOoX3PQA="
	m10ComboPubkey = "BF+pjkGM9bDnjjRIEt+3W82sMj5vfcOm5EVTKQnUUtpP5DZS/ranmGy5/pPmz3/CxvtjcGiCuRNdem78kNg85yM=:::jMotv64MoMcQZLniGahXEWsZsAK18ycvZ9t1rxFOqQuwfGchEcqrWCS0YR9hJrkPd2Ny7kQETGXsTDSNapaZJwE="
	m11ComboPubkey = "BOwo7bnxahUTL4DeSGo4ioZqocs1LjY7h9drAVP/HVHmXBD3WVDJrUIhzcIR3XpyzShw9RWxS0Oi788mEisVGSE=:::ZbPc7iPjBmHapOKNusHK8ux8791J65xQrgFTUGpEpqOWh2cF4bstR2q6QO5hi/SykqeVvw6pqiI22IIlP50EOwE="
)

func (r *Reactor) bootstrapCommitteeSize11() []*types.Validator {
	keys := []string{
		m01ComboPubkey, m02ComboPubkey, m03ComboPubkey, m04ComboPubkey, m05ComboPubkey, m06ComboPubkey, m07ComboPubkey, m08ComboPubkey, m09ComboPubkey, m10ComboPubkey, m11ComboPubkey,
	}
	committee := make([]*types.Validator, 0)
	for index, comboPubkey := range keys {
		pubkey, blsPubkey := r.blsCommon.SplitPubKey(comboPubkey)
		v := types.NewValidator("m0"+strconv.Itoa(index+1), meter.Address{}, *pubkey, *blsPubkey, 0)
		committee = append(committee, v)
	}
	return committee
}

func (r *Reactor) bootstrapCommitteeSize5() []*types.Validator {
	keys := []string{
		m01ComboPubkey, m02ComboPubkey, m03ComboPubkey, m04ComboPubkey, m05ComboPubkey,
	}
	committee := make([]*types.Validator, 0)
	for index, comboPubkey := range keys {
		pubkey, blsPubkey := r.blsCommon.SplitPubKey(comboPubkey)
		v := types.NewValidator("m0"+strconv.Itoa(index+1), meter.Address{}, *pubkey, *blsPubkey, 0)
		committee = append(committee, v)
	}
	return committee
}
