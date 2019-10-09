#!/bin/bash

PROJECT_NAME=meter-pos
VERSION=2.0.1

DOCKER_TAG=dfinlab/${PROJECT_NAME}:$VERSION
LATEST_TAG=dfinlab/${PROJECT_NAME}:latest

docker rmi -f dfinlab/${PROJECT_NAME}:$VERSION
docker build -t $DOCKER_TAG .
docker tag $DOCKER_TAG $LATEST_TAG
echo "Removed the temporary docker container"
echo "DONE."

docker push dfinlab/${PROJECT_NAME}