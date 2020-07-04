#!/bin/bash

# IMAGE_NAME=dfinlab/meter-pos
# VERSION=$(cat cmd/meter/VERSION)


# echo "Building ${IMAGE_NAME}"
# docker build -t $IMAGE_NAME:$VERSION .
# docker tag $IMAGE_NAME:$VERSION $IMAGE_NAME:latest
# docker push $IMAGE_NAME:$VERSION
# docker push $IMAGE_NAME:latest

# GIT_TAG=v$VERSION
# TEMP_CONTAINER_NAME=${PROJECT_NAME}-temp
# RELEASE_DIR=release/${PROJECT_NAME}-$VERSION-linux-amd64
# RELEASE_TARBALL=${PROJECT_NAME}-$VERSION-linux-amd64.tar.gz
# DEPENDENCY_TARBALL=${PROJECT_NAME}-$VERSION-linux-amd64-dependency.tar.gz


#docker run -d --name $TEMP_CONTAINER_NAME $DOCKER_TAG
#echo "Brought up a temporary docker container"
#mkdir -p $RELEASE_DIR/bin
# mkdir -p $RELEASE_DIR/lib
# docker cp $TEMP_CONTAINER_NAME:/usr/bin/meter $RELEASE_DIR/bin/
# docker cp $TEMP_CONTAINER_NAME:/usr/bin/disco $RELEASE_DIR/bin/
# docker cp $TEMP_CONTAINER_NAME:/usr/lib/libpbc.so $RELEASE_DIR/lib/
# docker cp $TEMP_CONTAINER_NAME:/usr/lib/libpbc.so.1 $RELEASE_DIR/lib/
# docker cp $TEMP_CONTAINER_NAME:/usr/lib/libpbc.so.1.0.0 $RELEASE_DIR/lib/
# docker rm --force $TEMP_CONTAINER_NAME
# echo "Removed the temporary docker container"

# cd $RELEASE_DIR/bin && tar -zcf ../$RELEASE_TARBALL . && cd -
# cd $RELEASE_DIR/lib && tar -zcf ../$DEPENDENCY_TARBALL . && cd -
# cp $RELEASE_DIR/$RELEASE_TARBALL release
# cp $RELEASE_DIR/$DEPENDENCY_TARBALL release
# rm -rf $RELEASE_DIR

# github-release release --user dfinlab --repo thor-consensus \
#     --tag ${GIT_TAG} --name "${GIT_TAG}" --pre-release
# echo "Created release ${GIT_TAG}"

# echo "Start upload release/${RELEASE_TARBALL}"
# github-release upload --user dfinlab --repo thor-consensus \
#     --tag ${GIT_TAG} --name "${RELEASE_TARBALL}" \
#     --file release/$RELEASE_TARBALL

# echo "Start upload release/${DEPENDENCY_TARBALL}"
# github-release upload --user dfinlab --repo thor-consensus \
#     --tag ${GIT_TAG} --name "${DEPENDENCY_TARBALL}" \
#     --file release/$DEPENDENCY_TARBALL
# echo "Release uploaded, please check https://github.com/dfinlab/thor-consensus/releases"
