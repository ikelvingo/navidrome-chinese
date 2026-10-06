#!/usr/bin/env bash
#
# Build the navidrome-chinese (CSE) Docker image with the project's branding convention.
#
# Convention (matches the historical builds):
#   GIT_TAG = v<VERSION>:cse
#   GIT_SHA = 中国特供<YYYYMMDD>
#   tags    = ikelvingo/navidrome-chinese:<VERSION>-cse  (+ :latest)
#
# Usage:
#   scripts/build-image.sh [VERSION] [options]
#
# Options:
#   --push              build and push to the registry named in the tag (Docker Hub)
#   --load              build and load into the local image store (default)
#   --no-cache          pass --no-cache to docker buildx (full rebuild)
#   --no-latest         do not also tag/push :latest
#   --provenance        keep buildx provenance attestation (default: disabled,
#                       so the tag is a plain single manifest, matching the
#                       historical 0.63.2 image and maximizing compatibility)
#   --platform <p>      build for a specific platform list, e.g. linux/amd64
#                       (multi-platform requires --push; cannot be combined with --load)
#   -h, --help          show this help
#
# Examples:
#   scripts/build-image.sh                      # local amd64 build, no push
#   scripts/build-image.sh 0.64.2 --load        # local build of version 0.64.2
#   scripts/build-image.sh 0.64.2 --no-cache --push          # publish :0.64.2-cse + :latest
#   scripts/build-image.sh 0.64.2 --platform linux/amd64,linux/arm64 --push
#
set -euo pipefail

REPO="ikelvingo/navidrome-chinese"
MODE="--load"
NO_CACHE=""
ADD_LATEST=1
PLATFORM=""
PROVENANCE=(--provenance=false)
VERSION=""

usage() { sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; }

while [ $# -gt 0 ]; do
  case "$1" in
    --push)      MODE="--push"; shift ;;
    --load)      MODE="--load"; shift ;;
    --no-cache)  NO_CACHE="--no-cache"; shift ;;
    --no-latest) ADD_LATEST=0; shift ;;
    --provenance) PROVENANCE=(); shift ;;
    --platform)  PLATFORM="${2:?--platform requires a value}"; shift 2 ;;
    -h|--help)   usage; exit 0 ;;
    -*)          echo "Unknown option: $1" >&2; usage >&2; exit 1 ;;
    *)           VERSION="$1"; shift ;;
  esac
done

# Always build from the repo root, no matter where the script is invoked from.
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
[ -f Dockerfile ] || { echo "Dockerfile not found in $ROOT" >&2; exit 1; }

if [ -z "$VERSION" ]; then
  VERSION="$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
fi
[ -n "$VERSION" ] || { echo "Could not determine VERSION; pass it explicitly." >&2; exit 1; }

BUILD_DATE="$(date +%Y%m%d)"
GIT_TAG="v${VERSION}:cse"
GIT_SHA="中国特供${BUILD_DATE}"

TAGS=(--tag "${REPO}:${VERSION}-cse")
[ "$ADD_LATEST" -eq 1 ] && TAGS+=(--tag "${REPO}:latest")

PLATFORM_ARG=()
if [ -n "$PLATFORM" ]; then
  PLATFORM_ARG=(--platform "${PLATFORM}")
  if [ "$MODE" = "--load" ] && [ "${PLATFORM}" != "${PLATFORM%%,*}" ]; then
    echo "ERROR: multi-platform builds require --push (cannot --load)." >&2
    exit 1
  fi
fi

echo "==> navidrome-chinese image build"
echo "    VERSION   = ${VERSION}"
echo "    GIT_TAG   = ${GIT_TAG}"
echo "    GIT_SHA   = ${GIT_SHA}"
echo "    tags      = ${TAGS[*]}"
echo "    mode      = ${MODE} ${NO_CACHE}"
echo "    platform  = ${PLATFORM:-<build host default>}"
echo

docker buildx build \
  ${NO_CACHE} \
  ${PLATFORM_ARG[@]+"${PLATFORM_ARG[@]}"} \
  ${PROVENANCE[@]+"${PROVENANCE[@]}"} \
  --build-arg GIT_TAG="${GIT_TAG}" \
  --build-arg GIT_SHA="${GIT_SHA}" \
  "${TAGS[@]}" \
  "${MODE}" \
  .

echo
echo "==> Done: ${REPO}:${VERSION}-cse"
if [ "$MODE" = "--load" ]; then
  echo "Verify: docker run --rm ${REPO}:${VERSION}-cse --version"
  echo "Expect: ${VERSION}:cse (${GIT_SHA})"
fi
