#!/bin/bash -x

if [ -z "$TAG" ]; then
  TAG="soulshack:dev"
fi

echo "building with tag: $TAG"

docker build . -t "$TAG"