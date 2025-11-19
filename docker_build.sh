#!/usr/bin/env bash
set -eu

image_name="go-app"
version="$1"

if [ -z "$version" ]; then
  echo "Add a version number"
  exit 1
fi

tag="${image_name}:${version}"
echo "building $tag"

if docker image ls --format "{{.Repository}}:{{.Tag}}"|grep "$tag"; then
    echo 'Image existed, use a different version'
else
    docker build -t "$tag" .
    docker tag "$tag" "${image_name}:latest"
    echo "successfully built latest image $tag"
fi



