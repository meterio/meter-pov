## Build Steps on Ubuntu

### install golang (1.14)

```
sudo add-apt-repository ppa:longsleep/golang-backports
sudo apt-get update
sudo apt-get install golang-1.14-go
```

Remember to add /usr/lib/go-1.14/bin to your \$PATH

### install dep (Optional)

```
sudo apt install go-dep
```

### install libgmp3

```
sudo apt install libgmp3-dev
```

### get the meter
```
git clone https://github.com/dfinlab/meter-pov-consensus.git

//our next example is network warringstakes. make sure switch to release branch!!!
git checkout release
```

### build the meter

make sure you have `$GOPATH` configured

```bash
# download dependencies
make dep

# build
make or make all
```
binary is located at ./bin/meter.

#### running meter
Let's use an example as running meter in network "Herd", one of meter's internal testnet. Discover server is used in this testnet, so we do not need to configure peers.

invoke the following command. meter is running... 
```
./meter --network warringstakes --verbosity 3 --api-addr 0.0.0.0:8669 --api-cors *
```
