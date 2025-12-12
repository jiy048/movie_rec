#!/usr/bin/env bash
set -euo pipefail


YML_PATH="docker-image.yml"
BUILD_CONTEXT="."

# define command variables 
MODE="normal"
if [[ "${1-}" == "--bump" ]]; then
  MODE="bump"
fi

# find image name and tag in docker-image.yml
IMAGE_LINE=$(grep -E '^\s*image:' "$YML_PATH" | head -n1 || true)
echo "$IMAGE_LINE"
if [[ -z "$IMAGE_LINE" ]]; then
  echo "ERROR: cannot find 'image:' line in $YML_PATH" >&2
  exit 1
fi

# extract image name and base tag
IMAGE_REF=${IMAGE_LINE#*:}
IMAGE_REF=$(echo "$IMAGE_REF" | xargs)

IMAGE_NAME=${IMAGE_REF%%:*}   # go-app
BASE_TAG=${IMAGE_REF#*:}      # 1.0.1

echo "Config image from $YML_PATH:"
echo "  IMAGE_NAME = $IMAGE_NAME"
echo "  BASE_TAG   = $BASE_TAG"
echo "  MODE         = $MODE"

## if Dockerfile changes, build image with ./start_docker.sh --bump
if [[ "$MODE" == "bump" ]]; then
# find next version
  IFS='.' read -r major minor patch <<< "$BASE_TAG" || true
  patch=$((patch + 1))
  NEXT_BASE_TAG="${major}.${minor}.${patch}"

  TS=$(date +%Y%m%d%H%M%S)
  FULL_TAG="${NEXT_BASE_TAG}_${TS}"
  
   echo "building image: ${IMAGE_NAME}:${FULL_TAG}"
  docker build -t "${IMAGE_NAME}:${FULL_TAG}" "$BUILD_CONTEXT"
  docker run -it --rm \
  -p 8080:8080 \
  "${IMAGE_NAME}:${FULL_TAG}"
  exit 0
fi

#   find local images and check if anything matches base image
EXISTING_TAGS=$(docker images "$IMAGE_NAME" --format '{{.Tag}}' | grep "^${BASE_TAG}_" || true)


if [[ -z "$EXISTING_TAGS" ]]; then
  echo "No local image found for version '$BASE_TAG', need to build."

  TS=$(date +%Y%m%d%H%M%S)
  FULL_TAG="${BASE_TAG}_${TS}"
# build local image if no match and start a container
  echo "Building image: ${IMAGE_NAME}:${FULL_TAG}"
  docker build -t "${IMAGE_NAME}:${FULL_TAG}" "$BUILD_CONTEXT"

  echo "Running container from new image..."

# if found the base image, use the latest image to start a container
else
  echo "Found local images for version '$BASE_TAG':"
  printf '  %s\n' $EXISTING_TAGS

  
  LATEST_TAG=$(printf '%s\n' $EXISTING_TAGS | sort | tail -n1)

  echo "Using latest local image tag: ${LATEST_TAG}"
  echo "Running container..."

fi
  
docker run -it --rm \
  -p 8080:8080 \
  "${IMAGE_NAME}:${FULL_TAG}"
