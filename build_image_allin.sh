#!/bin/bash

FULL_IMAGE_NAME=dfinlab/meter-all-in-one
ABBR_IMAGE_NAME=dfinlab/meter-allin

VERSION=$(cat cmd/meter/VERSION)

echo "Building ${DOCKER_TAG}"
docker build -f ./allin.Dockerfile -t $FULL_IMAGE_NAME:$VERSION .
docker tag $FULL_IMAGE_NAME:$VERSION $FULL_IMAGE_NAME:latest
docker tag $FULL_IMAGE_NAME:$VERSION $ABBR_IMAGE_NAME:latest
docker tag $FULL_IMAGE_NAME:$VERSION $ABBR_IMAGE_NAME:$VERSION

docker push $FULL_IMAGE_NAME:latest
docker push $FULL_IMAGE_NAME:$VERSION
docker push $ABBR_IMAGE_NAME:latest
docker push $ABBR_IMAGE_NAME:$VERSION
