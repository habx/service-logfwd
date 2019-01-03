#!/bin/sh -ex
# Test it with:
# docker run -ti --rm -v $(pwd):/work -v /var/run/docker.sock:/var/run/docker.sock -w /work docker:18

if [ ! -f logfwd ]; then
    cp /workspace/logfwd .
fi

DOCKER_IMAGE_NAME=habx/logfwd

if [ "${CIRCLE_TAG}" != "" ]; then # Tag names are cleaned up and simplified
    # Am I going to hell ?
    DOCKER_IMAGE_TAGS=$(echo ${CIRCLE_TAG} | grep -Eo "v([0-9]+\.){0}[0-9]+")' '$(echo ${CIRCLE_TAG} | grep -Eo "v([0-9]+\.){1}[0-9]+")' '$(echo ${CIRCLE_TAG} | grep -Eo "v([0-9]+\.){2}[0-9]+")
elif [ "${CIRCLE_BRANCH}" != "" ]; then # Branch names are cleaned up
    DOCKER_IMAGE_TAGS=$(echo ${CIRCLE_BRANCH} | sed "s/[^a-zA-Z0-9\.]/-/g")
else
    DOCKER_IMAGE_TAG=test
fi

docker build . -t ${DOCKER_IMAGE_NAME}:local

set +x # WARNING - SECRET
echo ${DOCKER_AUTH_PASS} | docker login -u ${DOCKER_AUTH_USER} --password-stdin
set -x

for tag in $DOCKER_IMAGE_TAGS
do
  docker tag ${DOCKER_IMAGE_NAME}:local ${DOCKER_IMAGE_NAME}:${tag}
  docker push ${DOCKER_IMAGE_NAME}:${tag}
done
