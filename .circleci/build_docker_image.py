#!/usr/bin/env python
"""
docker run -ti --rm -v $(pwd):/work -v /var/run/docker.sock:/var/run/docker.sock -w /work\
circleci/golang:1.11
"""

import os
import subprocess
import re
import logging

logging.basicConfig(
    format='%(asctime)-15s | %(lineno)-5d | %(levelname).4s | %(message)s',
    level=logging.DEBUG
)

if not os.path.isfile('logfwd'):
    subprocess.check_output(["cp", "/tmp/workspace/logfwd", "."])

DOCKER_IMAGE_NAME = 'habx/logfwd'

tags = []

# If it's tag, we're adding three docker tags with each level of version
if os.getenv('CIRCLE_TAG'):
    MATCH = re.match(r'^v([0-9]+)\.([0-9]+)\.([0-9]+)$', os.getenv('CIRCLE_TAG'))
    if MATCH:
        G = MATCH.groups()
        tags = [
            "%s.%s.%s" % (G[0], G[1], G[2]),
            "%s.%s" % (G[0], G[1]),
            "%s" % G[0],
        ]
# If it's a branch, we simply replace strange characters
elif os.getenv('CIRCLE_BRANCH'):
    tags = [re.sub(r'[^a-zA-Z0-9\.]', "-", os.getenv('CIRCLE_BRANCH'))]
# Otherwise, it's a test
else:
    tags = ["test"]

logging.info("Tags: %s", tags)

logging.info("Authentication...")
subprocess.check_output(["/bin/sh", "-c", "docker build . -t %s:local" % DOCKER_IMAGE_NAME])
subprocess.check_output([
    "/bin/sh",
    "-c",
    "echo %s | docker login -u %s --password-stdin" % (
        os.getenv('DOCKER_AUTH_PASS'),
        os.getenv('DOCKER_AUTH_USER'),
    )
])
logging.debug("Authentication... DONE")


for tag in tags:
    logging.info("Pushing %s", tag)
    subprocess.check_output([
        "/bin/sh",
        "-c",
        "docker tag %s:local %s:%s" % (DOCKER_IMAGE_NAME, DOCKER_IMAGE_NAME, tag)
    ])
    subprocess.check_output([
        "/bin/sh",
        "-c",
        "docker push %s:%s" % (DOCKER_IMAGE_NAME, tag)
    ])
