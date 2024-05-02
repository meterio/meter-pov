# Meter POV

Meter is the most decentralized and fasted Ethereum sidechain network with a native metastable gas currency

This is the first implementation written in golang.

## Table of contents

- [Installation](#installation)
  - [Requirements](#requirements)
  - [Building](#building)
- [Running Meter](#running-meter)
  - [Sub-commands](#sub-commands)
- [Docker](#docker)
- [Explorers](#explorers)
- [Faucet](#testnet-faucet)
- [RESTful API](#api)
- [Acknowledgement](#acknowledgement)
- [Contributing](#contributing)

## Installation

### Requirements

Meter requires `Go` 1.18+ and `C` compiler to build. To install `Go`, follow this [link](https://golang.org/doc/install).

### Building

Clone the meter-pov repo:

```
git clone https://github.com/meterio/meter-pov.git
cd meter-pov
```

Simply run:

```
make all
```

## Running

Connect to Meter's mainnet:

```
bin/meter --network main
```

Connect to Meter's testnet:

```
bin/meter --network warringstakes
```

To find out usages of all command line options:

```
bin/meter -h
```

- `--network value` the network to join (main|test)
- `--data-dir value` directory for block-chain databases
- `--beneficiary value` address for block rewards
- `--api-addr value` API service listening address (default: "localhost:8669")
- `--api-cors value` comma separated list of domains from which to accept cross origin requests to API
- `--verbosity value` log verbosity (0-9) (default: 0)
- `--max-peers value` maximum number of P2P network peers (P2P network disabled if set to 0) (default: 25)
- `--p2p-port value` P2P network listening port (default: 11235)
- `--nat value` port mapping mechanism (any|none|upnp|pmp|extip:<IP>) (default: "none")
- `--help, -h` show help
- `--version, -v` print the version

### Sub-commands

- `master-key` import and export master key

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

Once `meter` started, online _OpenAPI_ doc can be accessed in your browser. e.g. http://localhost:8669/ by default.

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

## Running a devnet with 2 nodes

Let's assume you have 2 nodes with different IPs, you'll need to enable these ports:
* powpool and api on port `8668`, optional if you run PoS chain only
* RESTful API on port `8669`
* observe server for prometheus metrics, probe on port `8670`, this port is also used for consensus message passing
* p2p port `11235` with UDP
* discover server port `55555` with UDP

The next few steps will show you how to setup a devnet with 2 nodes (named as `dev1` and `dev2`)

1. build the binary by `make all` and copy `./bin/meter` to `dev1` and `dev2` (take a look at [build instruction](./BUILD.md) )
2. choose a folder for meter chain data, refered by `<meter_data>`
3. collect public key for each node by `./meter public-key --data-dir <meter_data>`
4. copy `./bin/disco` to `dev` and start the discovery server on it by `disco`, the log should look like this:

```
2024/05/02 17:25:20 INFO UDP listener up net=enode://1d795801b4b31911385b702cf49f432aaddebbd8eeb76f757801c74b34ea477264f8d96cd4a6cf1d0d9a5310dbcb6808b2f3dbcb90eb0141e7505ae81e63040d@[::]:55555
Running enode://1d795801b4b31911385b702cf49f432aaddebbd8eeb76f757801c74b34ea477264f8d96cd4a6cf1d0d9a5310dbcb6808b2f3dbcb90eb0141e7505ae81e63040d@[::]:55555
```

The `enode://xxxx@[::]:55555` part is the enode id, replace the `[::]` part with actual ip of `dev1`, ths is `<discovery-server-enode-string>`

5. now you need to prepare a config file named `delegates.json`, a sample file looks like this:

```
[
  {
    "name": "dev1",
    "address": "<any-valid-ethereum-address>",
    "pub_key": "<pubkey-of-dev1>",
    "voting_power": 100,
    "network_addr": {
      "ip": "<ip-of-dev1>",
      "port": 8670
    }
  },
  {
    "name": "dev-02",
    "address": "<any-valid-ethereum-address>",
    "pub_key": "<pubkey-of-dev2>",
    "voting_power": 100,
    "network_addr": {
      "ip": "<ip-of-dev2>",
      "port": 8670
    }
  }
]

```
put this file under `<meter_data>/` on `dev1` and `dev2`

6. now you're ready to boot up

on both nodes, execute 

```
./meter --network staging --disco-topic dev --data-dir /etc/pos --committee-min-size 2 --init-configured-delegates --disco-server <discover-server-enode-string>

```

7. Idealy, this should do the trick, nodes will find each other with the help of discovery server and get ready for proposing the first block.
That's it. If you want to see the debug log, use `--verbosity -4` to enable.

## License

Meter is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.html), also included
in _LICENSE_ file in repository.
