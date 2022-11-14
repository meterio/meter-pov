module github.com/meterio/meter-pov

go 1.13

require (
	github.com/beevik/ntp v0.2.0
	github.com/beorn7/perks v1.0.0 // indirect
	github.com/btcsuite/btcd v0.0.0-20190213025234-306aecffea32
	github.com/cespare/cp v1.1.1 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/dfinlab/go-amino v0.14.1
	github.com/elazarl/go-bindata-assetfs v1.0.0
	github.com/enzoh/go-bls v0.0.0-20180308225442-56f8c69eaff5
	github.com/ethereum/go-ethereum v1.9.25
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
	github.com/hashicorp/golang-lru v0.5.4
	github.com/holiman/bloomfilter/v2 v2.0.3
	github.com/inconshreveable/log15 v0.0.0-20180818164646-67afb5ed74ec
	github.com/mattn/go-isatty v0.0.5-0.20180830101745-3fb116b82035
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/mattn/go-tty v0.0.0-20181127064339-e4f871175a2f
	github.com/pborman/uuid v0.0.0-20180906182336-adf5a7427709
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/common v0.3.0 // indirect
	github.com/prometheus/procfs v0.0.0-20190412120340-e22ddced7142 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.1-0.20200815110645-5c35d600f0ca
	github.com/tendermint/go-amino v0.16.0 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/sys v0.0.0-20220926163933-8cfa568d3c25
	gopkg.in/karalabe/cookiejar.v2 v2.0.0-20150724131613-8dcd6a7f4951
	gopkg.in/olebedev/go-duktape.v3 v3.0.0-20200619000410-60c24ae608a6
	gopkg.in/urfave/cli.v1 v1.20.0
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/syndtr/goleveldb => github.com/vechain/goleveldb v1.0.1-0.20211222044317-26c94f2f8e7e
