#!/bin/bash -x

if [ -z "$TAG" ]; then
  TAG="dev"
fi

echo "building with tag: $TAG"

docker build . -t gcr.io/linksnaps/soulshack:$TAG --platform linux/amd64
docker push gcr.io/linksnaps/soulshack:$TAG
