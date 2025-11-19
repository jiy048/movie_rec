#!/usr/bin/env bash
set -e

image="go-app"
tag="${1:-latest}"

echo "starting a container on ${image}:${tag}"
docker run -it --rm \
  -p 8080:8080 \
  -v "$PWD":/app \
  -w /app \
  "${image}:${tag}" \
  sh
