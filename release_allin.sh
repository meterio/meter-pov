#!/bin/bash

PROJECT_NAME=meter-all-in-one
VERSION=$(cat cmd/meter/VERSION)

DOCKER_TAG=dfinlab/${PROJECT_NAME}:$VERSION
LATEST_TAG=dfinlab/${PROJECT_NAME}:latest
DOCKER_ALLIN_TAG=dfinlab/meter-allin:$VERSION
LATEST_ALLIN_TAG=dfinlab/meter-allin:latest

echo "Building ${DOCKER_TAG}"
docker build -f ./allin.Dockerfile -t $DOCKER_TAG .
docker tag $DOCKER_TAG $LATEST_TAG
docker tag $DOCKER_TAG $DOCKER_ALLIN_TAG
docker tag $DOCKER_TAG $LATEST_ALLIN_TAG
docker push $LATEST_TAG
docker push $DOCKER_TAG
docker push $DOCKER_ALLIN_TAG
docker push $LATEST_ALLIN_TAG
