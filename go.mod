module github.com/meterio/meter-pov

go 1.13

require (
	github.com/aristanetworks/goarista v0.0.0-20170210015632-ea17b1a17847 // indirect
	github.com/beevik/ntp v0.2.0
	github.com/beorn7/perks v1.0.0 // indirect
	github.com/btcsuite/btcd v0.0.0-20190213025234-306aecffea32
	github.com/cespare/cp v1.1.1 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/deckarep/golang-set v0.0.0-20180603214616-504e848d77ea // indirect
	github.com/dfinlab/go-amino v0.14.1
	github.com/elazarl/go-bindata-assetfs v1.0.0
	github.com/enzoh/go-bls v0.0.0-20180308225442-56f8c69eaff5
	github.com/ethereum/go-ethereum v1.8.17
	github.com/fortytw2/leaktest v1.3.0
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/stat v0.0.0-20181125101827-41a0da705a5b
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.1.0 // indirect
	github.com/gorilla/context v1.1.1 // indirect
	github.com/gorilla/handlers v1.4.1
	github.com/gorilla/mux v1.6.2
	github.com/gorilla/websocket v1.5.0
	github.com/hashicorp/golang-lru v0.5.0
	github.com/huin/goupnp v0.0.0-20161224104101-679507af18f3 // indirect
	github.com/inconshreveable/log15 v0.0.0-20180818164646-67afb5ed74ec
	github.com/jackpal/go-nat-pmp v1.0.2-0.20160603034137-1fa385a6f458 // indirect
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.4
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/mattn/go-tty v0.0.0-20181127064339-e4f871175a2f
	github.com/pborman/uuid v0.0.0-20180906182336-adf5a7427709
	github.com/pkg/errors v0.8.0
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/common v0.3.0 // indirect
	github.com/prometheus/procfs v0.0.0-20190412120340-e22ddced7142 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/rjeczalik/notify v0.9.1 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/syndtr/goleveldb v0.0.0-20181128100959-b001fa50d6b2
	github.com/tendermint/go-amino v0.16.0 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/sys v0.0.0-20200814200057-3d37ad5750ed
	gopkg.in/karalabe/cookiejar.v2 v2.0.0-20150724131613-8dcd6a7f4951
	gopkg.in/olebedev/go-duktape.v3 v3.0.0-20181125150206-ccb656ba24c2
	gopkg.in/urfave/cli.v1 v1.20.0
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/syndtr/goleveldb => github.com/vechain/goleveldb v1.0.1-0.20211222044317-26c94f2f8e7e
