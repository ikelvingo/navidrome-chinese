FROM --platform=$BUILDPLATFORM ghcr.nju.edu.cn/crazy-max/osxcross:14.5-debian AS osxcross

########################################################################################################################
# 1. 使用官方的 xx 工具镜像，避免自行克隆编译，显著加快速度
FROM --platform=$BUILDPLATFORM alpine:3.20 AS xx-build
# 2. 获取 osxcross (仅在构建 darwin 目标时需要)
# v1.9.0
ENV XX_VERSION=a5592eab7a57895e8d385394ff12241bc65ecd50
ENV LANG=en_US.UTF-8 LANGUAGE=en_US.UTF-8

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
FROM --platform=$BUILDPLATFORM alpine:3.20 AS taglib-build
ARG TARGETPLATFORM
ARG CROSS_TAGLIB_VERSION=2.2.0-1
ENV CROSS_TAGLIB_RELEASES_URL=https://github.com/navidrome/cross-taglib/releases/download/v${CROSS_TAGLIB_VERSION}/
ENV LANG=en_US.UTF-8 LANGUAGE=en_US.UTF-8

# wget in busybox can't follow redirects
RUN <<EOT
    apk add --no-cache wget
    apk add --no-cache ca-certificates openssl
    PLATFORM=$(echo ${TARGETPLATFORM} | tr '/' '-')
    FILE=taglib-${PLATFORM}.tar.gz

    DOWNLOAD_URL=${CROSS_TAGLIB_RELEASES_URL}${FILE}
    wget ${DOWNLOAD_URL}

    mkdir /taglib
    tar -xzf ${FILE} -C /taglib
EOT

########################################################################################################################
### Build Navidrome UI
FROM --platform=$BUILDPLATFORM node:lts-alpine AS ui
WORKDIR /app
# 优化：增加国内镜像源加速
RUN npm config set registry https://registry.npmmirror.com/

ENV LANG=en_US.UTF-8 LANGUAGE=en_US.UTF-8
ENV CGO_ENABLED=0
COPY ui/package.json ui/package-lock.json ./
COPY ui/bin/ ./bin/
RUN npm ci

# Build bundle
COPY ui/ ./
# RUN grep -rv "?" . > /dev/null || (echo "错误：源码中依然残留点号！" && exit 1)
# RUN grep -r "?" ./ || (echo "错误：前端产物里没有竖线?" && exit 1)
RUN npm run build -- --outDir=/build

FROM scratch AS ui-bundle
COPY --from=ui /build /build
#COPY --from=ui-builder /build ./ui/build



########################################################################################################################
### Build Navidrome binary
FROM --platform=$BUILDPLATFORM golang:1.25-trixie AS base
ADD sources.list /etc/apt/sources.list.d/debian.sources
RUN apt-get update && apt-get install -y clang lld
# 从官方镜像引入 xx 
COPY --from=xx / /
WORKDIR /workspace

FROM --platform=$BUILDPLATFORM base AS build
ARG TARGETPLATFORM
# 关键修复点 1：强制设置构建环境编码，解决 ' ? ' 乱码问题
ENV LANG=en_US.UTF-8 LANGUAGE=en_US.UTF-8

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
    --mount=from=taglib-build,target=/taglib,src=/taglib,ro <<EOT

    # Setup CGO cross-compilation environment
    xx-go --wrap
    export CGO_ENABLED=1
    export CGO_CFLAGS_ALLOW="--define-prefix"
    export PKG_CONFIG_PATH=/taglib/lib/pkgconfig
    cat $(go env GOENV)

    # Only Darwin (macOS) requires clang (default), Windows requires gcc, everything else can use any compiler.
    # So let's use gcc for everything except Darwin.
    if [ "$(xx-info os)" != "darwin" ]; then
        export CC=$(xx-info)-gcc
        export CXX=$(xx-info)-g++
        export LD_EXTRA="-extldflags '-static -latomic'"
    fi
    if [ "$(xx-info os)" = "windows" ]; then
        export EXT=".exe"
    fi

    go build -tags=netgo,sqlite_fts5 -ldflags="${LD_EXTRA} -w -s \
        -X github.com/navidrome/navidrome/consts.gitSha=${GIT_SHA} \
        -X github.com/navidrome/navidrome/consts.gitTag=${GIT_TAG}" \
        -o /out/navidrome${EXT} .
EOT

# Verify if the binary was built for the correct platform and it is statically linked
RUN xx-verify --static /out/navidrome*

FROM scratch AS binary
COPY --from=build /out /

########################################################################################################################
### Build Final Image
FROM alpine:3.20 AS final

# 优化：使用国内 Alpine 镜像源
RUN sed -i "s/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g" /etc/apk/repositories
# Install ffmpeg and mpv
RUN apk add -U --no-cache ffmpeg mpv sqlite
# RUN apk --no-cache add ca-certificates wget && \
#     wget -q -O /etc/apk/keys/sgerrand.rsa.pub https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub && \
#     wget https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.25-r0/glibc-2.25-r0.apk && \
#     wget https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.25-r0/glibc-bin-2.25-r0.apk && \
#     wget https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.25-r0/glibc-i18n-2.25-r0.apk 
# RUN apk add glibc-2.25-r0.apk glibc-bin-2.25-r0.apk glibc-i18n-2.25-r0.apk && rm -rf glibc-2.25-r0.apk glibc-bin-2.25-r0.apk glibc-i18n-2.25-r0.apk
# COPY ./locale.md /locale.md
# RUN cat locale.md|xargs -i /usr/glibc-compat/bin/localedef -i {} -f UTF-8 {}.UTF-8
ENV LANG=en_US.UTF-8 LANGUAGE=en_US.UTF-8

# 设置容器内时区
ENV TZ=Asia/Shanghai

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

