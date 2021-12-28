nvm install 12
nvm use 12
npm install -g solc@0.4.24
npm install -g sol-merger

node --version
solcjs --version

```
rm -rf ./build/

sol-merger ./meter.sol ./build
sol-merger ./executor.sol ./build
sol-merger ./extension.sol ./build
sol-merger ./measure.sol ./build
sol-merger ./params.sol ./build
sol-merger ./prototype.sol ./build
sol-merger ./meternative.sol ./build
sol-merger ./meter-erc20.sol ./build
```

```
rm -rf ./compiled/
cd build
solcjs --optimize-runs 200 --overwrite --bin-runtime --bin --abi -o ./compiled meter.sol executor.sol extension.sol measure.sol params.sol prototype.sol meternative.sol meter-erc20.sol
cd -
mv build/compiled .
```

```
cd compiled

for f in *; do echo "$f" ; done
for f in *; do mv "$f" "$(echo "$f" | sed 's/^.*sol_//')"; done
for f in *; do echo "$f" ; done

cd -
```

go-bindata -nometadata -ignore=_ -pkg gen -o bindata.go compiled/
