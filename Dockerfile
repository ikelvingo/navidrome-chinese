FROM --platform=$BUILDPLATFORM ghcr.io/crazy-max/osxcross:14.5-debian AS osxcross

########################################################################################################################
### Build xx (original image: tonistiigi/xx)
FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/alpine:3.20 AS xx-build

# v1.9.0
ENV XX_VERSION=a5592eab7a57895e8d385394ff12241bc65ecd50

RUN apk add -U --no-cache git
RUN git clone https://github.com/tonistiigi/xx && \
    cd xx && \
    git checkout ${XX_VERSION} && \
    mkdir -p /out && \
    cp src/xx-* /out/

RUN cd /out && \
    ln -s xx-cc /out/xx-clang && \
    ln -s xx-cc /out/xx-clang++ && \
    ln -s xx-cc /out/xx-c++ && \
    ln -s xx-apt /out/xx-apt-get

# xx mimics the original tonistiigi/xx image
FROM scratch AS xx
COPY --from=xx-build /out/ /usr/bin/

########################################################################################################################
### Get TagLib
FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/alpine:3.20 AS taglib-build
ARG TARGETPLATFORM
ARG CROSS_TAGLIB_VERSION=2.2.1-1
ENV CROSS_TAGLIB_RELEASES_URL=https://github.com/navidrome/cross-taglib/releases/download/v${CROSS_TAGLIB_VERSION}/

# wget in busybox can't follow redirects
RUN apk add --no-cache curl && \
    PLATFORM=$(echo ${TARGETPLATFORM} | tr '/' '-') && \
    FILE=taglib-${PLATFORM}.tar.gz && \
    DOWNLOAD_URL=${CROSS_TAGLIB_RELEASES_URL}${FILE} && \
    echo "Downloading TagLib from: ${DOWNLOAD_URL}" && \
    curl -L -f -o "${FILE}" "${DOWNLOAD_URL}" && \
    echo "Download completed" && \
    mkdir -p /taglib && \
    tar -xzf "${FILE}" -C /taglib

########################################################################################################################
### Build Navidrome UI
FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/node:lts-alpine AS ui
WORKDIR /app

# Install node dependencies
COPY ui/package.json ui/package-lock.json ./
COPY ui/bin/ ./bin/
RUN chmod +x bin/*.sh && npm ci

# Build bundle
COPY ui/ ./
RUN npm run build -- --outDir=/build

FROM scratch AS ui-bundle
COPY --from=ui /build /build

########################################################################################################################
### Build Navidrome binary
FROM --platform=$BUILDPLATFORM public.ecr.aws/docker/library/golang:1.25-trixie AS base
RUN apt-get update && apt-get install -y clang lld
COPY --from=xx / /
WORKDIR /workspace

FROM --platform=$BUILDPLATFORM base AS build

# Install build dependencies for the target platform
ARG TARGETPLATFORM

RUN xx-apt install -y binutils gcc g++ libc6-dev zlib1g-dev
RUN xx-verify --setup

RUN --mount=type=bind,source=. \
    --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

ARG GIT_SHA
ARG GIT_TAG

RUN --mount=type=bind,source=. \
    --mount=from=ui,source=/build,target=./ui/build,ro \
    --mount=from=osxcross,src=/osxcross/SDK,target=/xx-sdk,ro \
    --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=from=taglib-build,target=/taglib,src=/taglib,ro \
    sh -c '\
    set -ex; \
    # Setup cross-compilation environment \
    export CGO_ENABLED=1; \
    export CGO_CFLAGS_ALLOW="--define-prefix"; \
    export PKG_CONFIG_PATH=/taglib/lib/pkgconfig; \
    # Get target OS and architecture \
    OS="$(xx-info os)"; \
    ARCH="$(xx-info arch)"; \
    echo "Building for OS: $OS, ARCH: $ARCH"; \
    # Set compiler based on OS \
    if [ "$OS" = "darwin" ]; then \
        # Darwin uses clang \
        export CC="clang"; \
        export CXX="clang++"; \
        export LD_EXTRA=""; \
    elif [ "$OS" = "windows" ]; then \
        # Windows uses gcc and adds .exe extension \
        export CC="$(xx-info)-gcc"; \
        export CXX="$(xx-info)-g++"; \
        export LD_EXTRA="-extldflags \"-static -latomic\""; \
        export EXT=".exe"; \
    else \
        # Linux and others use gcc \
        export CC="$(xx-info)-gcc"; \
        export CXX="$(xx-info)-g++"; \
        export LD_EXTRA="-extldflags \"-static -latomic\""; \
        export EXT=""; \
    fi; \
    echo "Using CC: $CC, CXX: $CXX"; \
    # Build the binary \
    go build -tags=netgo,sqlite_fts5 -ldflags="${LD_EXTRA} -w -s \
        -X github.com/ikelvingo/navidrome/consts.gitSha=${GIT_SHA} \
        -X github.com/ikelvingo/navidrome/consts.gitTag=${GIT_TAG}" \
        -o /out/navidrome${EXT} .'

# Verify if the binary was built for the correct platform and it is statically linked
RUN xx-verify --static /out/navidrome*

FROM scratch AS binary
COPY --from=build /out /

########################################################################################################################
### Build Final Image
FROM public.ecr.aws/docker/library/alpine:3.20 AS final
LABEL maintainer="deluan@navidrome.org"
LABEL org.opencontainers.image.source="https://github.com/navidrome/navidrome"

# Install ffmpeg and mpv
RUN apk add -U --no-cache ffmpeg mpv sqlite

# Copy navidrome binary
COPY --from=build /out/navidrome /app/

VOLUME ["/data", "/music"]
ENV ND_MUSICFOLDER=/music
ENV ND_DATAFOLDER=/data
ENV ND_CONFIGFILE=/data/navidrome.toml
ENV ND_PORT=4533
RUN touch /.nddockerenv

EXPOSE ${ND_PORT}
WORKDIR /app

ENTRYPOINT ["/app/navidrome"]
