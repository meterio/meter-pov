#!/bin/bash

# this script build docker image for these repo:
# pos-only image: dfinlab/meter-pos
# full image: dfinlab/meter-allin & dfinlab/meter-all-in-one
# with two tags: $version & latest

VERSION=$(cat cmd/meter/VERSION)

POS_IMAGE_NAME=dfinlab/meter-pos
POW_IMAGE_NAME=dfinlab/meter-pow
POS_DOCKERFILE=_docker/pos.Dockerfile

FULL_IMAGE_NAME=dfinlab/meter-allin
LEGACY_IMAGE_NAME=dfinlab/meter-all-in-one
FULL_DOCKERFILE=_docker/testnet.Dockerfile

echo "Building ${POS_IMAGE_NAME}"
docker build -f $POS_DOCKERFILE -t $POS_IMAGE_NAME:$VERSION .
docker tag $POS_IMAGE_NAME:$VERSION $POS_IMAGE_NAME:latest

docker push $POS_IMAGE_NAME:$VERSION
docker push $POS_IMAGE_NAME:latest

docker pull $POS_IMAGE_NAME:latest
docker pull $POW_IMAGE_NAME:latest

echo "Building ${FULL_IMAGE_NAME}"
docker build -f $FULL_DOCKERFILE -t $FULL_IMAGE_NAME:$VERSION .
docker tag $FULL_IMAGE_NAME:$VERSION $FULL_IMAGE_NAME:latest
docker tag $FULL_IMAGE_NAME:$VERSION $LEGACY_IMAGE_NAME:latest
docker tag $FULL_IMAGE_NAME:$VERSION $LEGACY_IMAGE_NAME:$VERSION

docker push $FULL_IMAGE_NAME:latest
docker push $FULL_IMAGE_NAME:$VERSION

docker push $LEGACY_IMAGE_NAME:latest
docker push $LEGACY_IMAGE_NAME:$VERSION