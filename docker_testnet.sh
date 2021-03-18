#!/bin/bash

# this script build docker image for these repo:
# pos-only image: meterio/pos
# full image: meterio/mainnet
# with two tags (latest & $version)

VERSION=$(cat cmd/meter/VERSION)
POS_DOCKER_REPO=dfinlab/meter-pos
POW_DOCKER_RPEO=dfinlab/meter-pow
POW_STATIC_TAG=$POW_DOCKER_RPEO:mainnet

POS_DOCKERFILE=_docker/pos.Dockerfile
POS_VERSION_TAG=$POS_DOCKER_REPO:$VERSION
POS_STATIC_TAG=$POS_DOCKER_REPO:mainnet

FULL_DOCKER_REPO=dfinlab/mainnet-allin
FULL_DOCKERFILE=_docker/mainnet.Dockerfile
FULL_VERSION_TAG=$FULL_DOCKER_REPO:$VERSION
FULL_STATIC_TAG=$FULL_DOCKER_REPO:latest

echo "Building docker image $POS_VERSION_TAG & $POS_STATIC_TAG"
docker build -f $POS_DOCKERFILE -t $POS_VERSION_TAG .
docker tag $POS_VERSION_TAG $POS_STATIC_TAG

echo "Push to DockerHub with tags: $POS_VERSION_TAG & $POS_STATIC_TAG"
docker push $POS_VERSION_TAG
docker push $POS_STATIC_TAG

echo "Pull latest docker image with tags: $POS_STATIC_TAG & $POW_STATIC_TAG"
docker pull $POS_STATIC_TAG
docker pull $POW_STATIC_TAG

echo "Building docker image: $FULL_VERSION_TAG $FULL_STATIC_TAG"
docker build -f $FULL_DOCKERFILE -t $FULL_VERSION_TAG .
docker tag $FULL_VERSION_TAG $FULL_STATIC_TAG

docker push $FULL_STATIC_TAG
docker push $FULL_VERSION_TAG
