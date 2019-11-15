## Build Steps on Ubuntu

### install golang (1.12)

```
sudo add-apt-repository ppa:longsleep/golang-backports
sudo apt-get update
sudo apt-get install golang-1.12-go
```

Remember to add /usr/lib/go-1.12/bin to your \$PATH

### install dep (Optional)

```
sudo apt install go-dep
```

### install libgmp3

```
sudo apt install libgmp3-dev
```

### build

make sure you have `$GOPATH` configured

```bash
# download dependencies
make dep

# build
make
```
