#!/bin/bash

VERSION=$(cat cmd/meter/VERSION)
GIT_COMMIT=$(git --no-pager log --pretty="%h" -n 1)
GIT_TAG=$(git tag -l --points-at HEAD)

if [ -z $GIT_TAG ]
then
    FULL_VERSION=${VERSION}-${GIT_COMMIT}-dev
else
    FULL_VERSION=${VERSION}-${GIT_COMMIT}-release
fi

echo "Full version is ${FULL_VERSION}"

docker pull meterio/mainnet-pow:24.04
docker pull meterio/bitcoind-exporter:24.04
docker pull meterio/run-env:24.04
docker pull meterio/build-env:24.04

# get ready for a fallback
docker pull meterio/mainnet:latest
docker tag meterio/mainnet:latest meterio/mainnet:fallback

# NOTICE: enable these lines if you need to upgrade gear version
# echo "Building run-env image with tag: 24.04"
# docker build -f _docker/run-env.Dockerfile -t meterio/run-env:24.04 .
# docker push meterio/run-env:24.04

echo "Building mainnet image with tags: tesla and 24.04"
docker build --platform=linux/amd64 -f _docker/mainnet.Dockerfile -t meterio/mainnet:tesla .
docker tag meterio/mainnet:tesla meterio/mainnet:latest
docker tag meterio/mainnet:tesla meterio/mainnet:${FULL_VERSION}

docker push meterio/mainnet:tesla

# USE WITH CAUTION: this will trigger a full network reboot
# docker push meterio/mainnet:latest
