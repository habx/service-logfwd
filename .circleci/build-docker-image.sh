#!/bin/sh -ex

env

if [ ! -f logfwd ]; then
    cp /workspace/logfwd .
fi

DOCKER_IMAGE_NAME=habx/logfwd

if [ -n "${CIRCLE_TAG}" ]; then
    DOCKER_IMAGE_TAG=${CIRCLE_TAG}
elif [ -n "${CIRCLE_BRANCH}" ]; then
    DOCKER_IMAGE_TAG=${CIRCLE_BRANCH}
else
    DOCKER_IMAGE_TAG=test
fi

DOCKER_IMAGE_FULL=${DOCKER_IMAGE_NAME}:${DOCKER_IMAGE_TAG}

docker build . -t ${DOCKER_IMAGE_FULL}
docker push ${DOCKER_IMAGE_FULL}
echo ${DOCKER_AUTH_PASS} | docker login -u ${DOCKER_AUTH_USER} --password-stdin
