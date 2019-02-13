## Build Steps on Ubuntu

### install golang (1.11)

```
sudo add-apt-repository ppa:gophers/archive
sudo apt-get update
sudo apt-get install golang-1.10-go
```

Remember to add /usr/lib/go-1.10/bin to your $PATH

### install dep

```
sudo apt install go-dep
```

### install libgmp3

```
sudo apt install libgmp3-dev
```


### build

make sure you have $GOPATH configured
make sure you install dep before you execute `make dep`, otherwise you'll get errors about dependencies such as go-amino and rs/cors

```
cd $GOPATH/src
mkdir -p github.com/dfinlab
cd github.com/dfinlab
git clone https://github.com/dfinlab/go-amino

cd $GOPATH/src
git clone https://github.com/dfinlab/thor-consensus
cd thor-consensus
make dep
make
```
