# Meter POV

Meter is the most decentralized and fasted Ethereum sidechain network with a native metastable gas currency

This is the first implementation written in golang.

## Table of contents

* [Installation](#installation)
    * [Requirements](#requirements)
    * [Getting the source](#getting-the-source)
    * [Dependency management](#dependency-management)
    * [Building](#building)
* [Running Meter](#running-meter)
    * [Sub-commands](#sub-commands)
* [Docker](#docker)
* [Explorers](#explorers)
* [Faucet](#testnet-faucet)
* [RESTful API](#api)
* [Acknowledgement](#acknowledgement)
* [Contributing](#contributing)

## Installation

### Requirements

Meter requires `Go` 1.12+ and `C` compiler to build. To install `Go`, follow this [link](https://golang.org/doc/install). 


### Getting the source

Clone the meter-pov repo:

```
git clone https://github.com/meterio/meter-pov.git
cd meter-pov
```


### Dependency management

Simply run:
```
make dep
```

To manually install dependencies, choices are

- [dep](https://github.com/golang/dep), Golang's official dependency management tool 

    ```
    dep ensure -vendor-only
    ```
    (*Note that to make `dep` work, you should put the source code at `$GOPATH/src/github.com/dfinlab/meter`*)

- git submodule

    ```
    git submodule update --init
    ```

### Building

To build the main app `meter`, just run

```
make
```

#### Tips

You might encounter problems like this during `make`:

```
vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/curve.go:43:44: fatal error: libsecp256k1/include/secp256k1.h: No such file or directory
 #include "libsecp256k1/include/secp256k1.h"
                                            ^
compilation terminated.
```

Then solution will be like this: 
```
go get github.com/ethereum/go-ethereum
cp -r \
  "${GOPATH}/src/github.com/ethereum/go-ethereum/crypto/secp256k1/libsecp256k1" \
  "vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/"

```


or build the full suite:

```
make all
```

If no error reported, all built executable binaries will appear in folder *bin*.

## Running Meter

Connect to Meter's mainnet:

```
bin/meter --network main
```


Connect to VeChain's testnet:

```
bin/meter --network warringstakes
```


To find out usages of all command line options:

```
bin/meter -h
```

- `--network value`        the network to join (main|test)
- `--data-dir value`       directory for block-chain databases
- `--beneficiary value`    address for block rewards
- `--api-addr value`       API service listening address (default: "localhost:8669")
- `--api-cors value`       comma separated list of domains from which to accept cross origin requests to API
- `--verbosity value`      log verbosity (0-9) (default: 3)
- `--max-peers value`      maximum number of P2P network peers (P2P network disabled if set to 0) (default: 25)
- `--p2p-port value`       P2P network listening port (default: 11235)
- `--nat value`            port mapping mechanism (any|none|upnp|pmp|extip:<IP>) (default: "none")
- `--help, -h`             show help
- `--version, -v`          print the version
- `--force-last-kframe`    force the node to take nonce from last k-block, you don't need this when you start the node with genesis block
- `--gen-kframe`           periodically generate k-block data
- `--skip-signature-check` skip the signature check (ONLY for debug)

### Sub-commands

- `master-key`          import and export master key

```
# export master key to keystore
bin/meter master-key --export > keystore.json


# import master key from keystore
cat keystore.json | bin/meter master-key --import
```

## Docker

Docker is one quick way for running a meter node:

```
docker run --network host --name meter -e NETWORK="main" -v /home/ubuntu/meter-main:/pos -d meterio/mainnet:latest
```

Do not forget to add the `--api-addr 0.0.0.0:8669` flag if you want other containers and/or hosts to have access to the RESTful API. `Meter` binds to `localhost` by default and it will not accept requests outside the container itself without the flag.

The [Dockerfile](_docker/mainnet.Dockerfile) is designed to build the last release of the source code and will publish docker images to [dockerhub](https://hub.docker.com/r/meterio/mainnet/) by release, feel free to fork and build Dockerfile for your own purpose.

## Explorers

Awesome explorers built by the meter team:

- [Testnet Explorer](https://scan-warringstakes.meter.io/)
- [Mainnet Explorer](https://scan.meter.io/)

## Faucet

- [Testnet Faucet](https://faucet-warringstakes.meter.io/)
- [Mainnet Faucet](https://faucet.meter.io/)

## API

Once `meter` started, online *OpenAPI* doc can be accessed in your browser. e.g. http://localhost:8669/ by default.

[![Meter Restful API](meter-rest.png)](http://localhost:8669/)

## Acknowledgement

A Special shout out to following projects:

- [Ethereum](https://github.com/ethereum)

- [Swagger](https://github.com/swagger-api)

## Contributing

Thanks you so much for considering to help out with the source code! We welcome contributions from anyone on the internet, and are grateful for even the smallest of fixes!

Please fork, fix, commit and send a pull request for the maintainers to review and merge into the main code base.

### Forking Meter
When you "Fork" the project, GitHub will make a copy of the project that is entirely yours; it lives in your namespace, and you can push to it.

### Getting ready for a pull request
Please check the following:

- Code must be adhere to the official Go Formatting guidelines.
- Get the branch up to date, by merging in any recent changes from the master branch.

### Making the pull request
- On the GitHub site, go to "Code". Then click the green "Compare and Review" button. Your branch is probably in the "Example Comparisons" list, so click on it. If not, select it for the "compare" branch.
- Make sure you are comparing your new branch to master. It probably won't be, since the front page is the latest release branch, rather than master now. So click the base branch and change it to master.
- Press Create Pull Request button.
- Give a brief title.
- Explain the major changes you are asking to be code reviewed. Often it is useful to open a second tab in your browser where you can look through the diff yourself to remind yourself of all the changes you have made.


## Deployment Steps

Right now, only node mode is supported. solo mode is not supported anymore. The next few steps will show you how to setup a cluster with 2 nodes (named as `node1` and `node2`)

1. prepare the binary and copy it to `node1` and `node2` (take a look at [build instruction](./BUILD.md) )
2. run the binary first with `./bin/meter --network test --verbosity 9`
3. take a note on the enode id for `node1`, you could find it in log here:

```
INFO[03-30|10:53:15] start up                                 pkg=p2psrv self="enode://0f73e6fad8ee070940d50ad326067c096696865398843e09d837a11ae2006a1df8688d01e0bd93a4c2db4253e571a514fc6f7c03e16c0b588044fdd082c2a145@[::]:11235?discport=0"
```

The `enode://xxxx@[::]:11235` part is the enode, replace the `[::]` part with actual ip of `node1`

4. now you need to prepare a config file named `delegates.json`, a sample file looks like this:

```
[
    {
        "address" : "a0da10fa4e8e810c4ef3f9a3cd8091a7b10e05fd",
        "pub_key" : "node1-pub-key",
        "voting_power" : "90",
        "network_addr" : {
            "id" : "1000",
            "ip" : "node1-ip",
            "port" : 8080,
            "name" : "nod1"
        },
        "accum" : "7689255112978"
    },
    {
        "address" : "d4be94fda23e12ee656b5123c9ac80bbf81971a5",
        "pub_key" : "node2-pub-key",
        "voting_power" : "100",
        "network_addr" : {
            "id" : "1001",
            "ip" : "node2-ip",
            "port" : 8080,
            "name" : "node2"
        },
        "accum" : "2057045235008"
    }
]

```

replace node1-ip with ip of `node1`
replace node1-pub-key with content from `~/.com.dfinlab.meter/public.key` on `node1`
replace node2-ip with ip of `node2`
replace node2-pub-key with content from `~/.com.dfinlab.meter/public.key` on `node2`

5. copy `delegates.json` on to `node1` and `node2`, place it under `~/.com.dfinlab.meter/delegates.json`

6. now you're ready to boot up

on `node1`, use `./bin/meter --network test --verbosity 9`
on `node2`, use `./bin/meter --network test --verbosity 9 --peers [enode-id-of-node1]`

7. if you see from the log that block has been generated with consensus, everything is up and running!

A sample log for commited block:

```
INFO[02-12|14:45:40]
===========================================================
Block commited at height 1
===========================================================
 pkg=consensus elapsedTime=707218 bestBlockHeight=1

Block(504 B){
BlockHeader: Header(0x0000055d29c1adde910544507f7b712370f2a91dcc4b34d21f3b696e4c10f515):
        Number:                 1
        ParentID:               0x0000055c3a8556200e83ac9fe80e0323de76f529edeb150613d2af5166dfc12d
        Timestamp:              1550011540
        Signer:                 0x7280d2f760ce59c5a2e0bb04eccdef45c6bb7d14
        Beneficiary:            0x7280d2f760ce59c5a2e0bb04eccdef45c6bb7d14
        BlockType:              2
        LastKBlockHieght:       0
        GasLimit:               10000000
        GasUsed:                0
        TotalScore:             1
        TxsRoot:                0x45b0cfc220ceec5b7c1c62c4d4193d38e4eba48e8815729ce75f9c0ab0e4c1c0
        StateRoot:              0x7d7cb8edd7a420953caad0a9a90583dd6fc6fc0a52235f2d801ee16d43a3f389
        ReceiptsRoot:   0x45b0cfc220ceec5b7c1c62c4d4193d38e4eba48e8815729ce75f9c0ab0e4c1c0
        Signature:              0xfca190d1e2388e52d290ec91777c3e4b2db7b6273ddfd27c344062d14355c92b56d69a2c5da2da39a69e43425c0fa5402928bb41315c0f87fe299bd7820e08db01,
Transactions: [],
KBlockData: {0 0x0000000000000000000000000000000000000000 []},
CommitteeInfo: {[] [] []}
}
```

That's it. The next time, you won't need the `--peers` flag for `node2` any more, since it's already stored in cache. Enjoy!

## License

Meter is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.html), also included
in *LICENSE* file in repository.
