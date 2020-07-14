#!/bin/bash

# this script build docker image for these repo:
# pos-only image: meterio/pos
# full image: meterio/mainnet
# with two tags (latest & $version)

VERSION=$(cat cmd/meter/VERSION)
POS_IMAGE_NAME=meterio/pos
POW_IMAGE_NAME=meterio/pow
POS_DOCKERFILE=_docker/pos.Dockerfile

FULL_IMAGE_NAME=meterio/mainnet
FULL_DOCKERFILE=_docker/mainnet.Dockerfile

echo "Building ${POS_IMAGE_NAME}"
docker build -f $POS_DOCKERFILE -t $POS_IMAGE_NAME:$VERSION .
docker tag $POS_IMAGE_NAME:$VERSION $POS_IMAGE_NAME:latest

docker push $POS_IMAGE_NAME:$VERSION
docker push $POS_IMAGE_NAME:latest

docker pull meterio/pos:latest
docker pull meterio/pow:latest

echo "Building ${FULL_IMAGE_NAME}"
docker build -f $FULL_DOCKERFILE -t $FULL_IMAGE_NAME:$VERSION .
docker tag $FULL_IMAGE_NAME:$VERSION $FULL_IMAGE_NAME:latest

docker push $FULL_IMAGE_NAME:latest
docker push $FULL_IMAGE_NAME:$VERSION
