#!/bin/bash

POS_IMAGE_NAME=meterio/pos
POW_IMAGE_NAME=meterio/pow
VERSION=$(cat cmd/meter/VERSION)
FULL_IMAGE_NAME=meterio/mainnet


echo "Building ${POS_IMAGE_NAME}"
docker build -t $POS_IMAGE_NAME:$VERSION .
docker tag $POS_IMAGE_NAME:$VERSION $POS_IMAGE_NAME:latest

docker push $POS_IMAGE_NAME:$VERSION
docker push $POS_IMAGE_NAME:latest

docker pull meterio/pos:latest
docker pull meterio/pow:latest

echo "Building ${FULL_IMAGE_NAME}"
docker build -f ./main.Dockerfile -t $FULL_IMAGE_NAME:$VERSION .
docker tag $FULL_IMAGE_NAME:$VERSION $FULL_IMAGE_NAME:latest

docker push $FULL_IMAGE_NAME:latest
docker push $FULL_IMAGE_NAME:$VERSION
