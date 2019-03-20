#!/bin/bash

PROJECT_NAME=meter-pos
VERSION=1.0

DOCKER_TAG=dfinlab/${PROJECT_NAME}:$VERSION
GIT_TAG=v$VERSION
TEMP_CONTAINER_NAME=${PROJECT_NAME}-temp
RELEASE_TARBALL=${PROJECT_NAME}-$VERSION-linux-amd64.tar.gz
RELEASE_DIR=release/${PROJECT_NAME}-$VERSION-linux-amd64

# docker build -t $DOCKER_TAG .
docker run -d --name $TEMP_CONTAINER_NAME $DOCKER_TAG
echo "Brought up a temporary docker container"
mkdir -p $RELEASE_DIR
docker cp $TEMP_CONTAINER_NAME:/usr/local/bin/thor $RELEASE_DIR/
docker rm --force $TEMP_CONTAINER_NAME
echo "Removed the temporary docker container"


cd $RELEASE_DIR && tar -zcf ../$RELEASE_TARBALL . && cd -
rm -rf $RELEASE_DIR

github-release release --user dfinlab --repo thor-consensus \
    --tag ${GIT_TAG} --name "${GIT_TAG}" --pre-release
echo "Created release ${GIT_TAG}"

echo "Start upload release/${RELEASE_TARBALL}"
github-release upload --user dfinlab --repo thor-consensus \
    --tag ${GIT_TAG} --name "${RELEASE_TARBALL}" \
    --file release/$RELEASE_TARBALL

echo "Release uploaded, please check https://github.com/dfinlab/thor-consensus/releases"
