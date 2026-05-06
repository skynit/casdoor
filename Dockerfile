FROM --platform=$BUILDPLATFORM node:20.18.0 AS FRONT
WORKDIR /web

# 1. 先只复制依赖文件，利用缓存层
COPY ./web/package.json ./web/yarn.lock ./
RUN yarn install --frozen-lockfile --network-timeout 1000000

# 2. 再复制全部源码（会覆盖上面的 package.json）
COPY ./web .

# 3. COPY 之后再 sed，才能真正生效
RUN sed -i '/"postbuild":/d' package.json

# 4. 构建
RUN NODE_OPTIONS="--max-old-space-size=4096" yarn run build \
    && mv build-temp build  

FROM --platform=$BUILDPLATFORM golang:1.25.8 AS BACK
WORKDIR /go/src/casdoor

COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .

# 构建二进制
RUN ./build.sh

FROM alpine:latest AS STANDARD
LABEL MAINTAINER="https://casdoor.org/"
ARG USER=casdoor
ARG TARGETOS
ARG TARGETARCH
ENV BUILDX_ARCH="${TARGETOS:-linux}_${TARGETARCH:-amd64}"
ENV CASDOOR_CONF=/conf/app.conf   

RUN sed -i 's/https/http/' /etc/apk/repositories
RUN apk add --update sudo tzdata curl ca-certificates && update-ca-certificates

# 创建用户及必要的目录（与服务器挂载点对齐）
RUN adduser -D $USER -u 1000 \
    && echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
    && chmod 0440 /etc/sudoers.d/$USER \
    && mkdir -p /conf /logs /files /data \
    && chown -R $USER:$USER /conf /logs /files /data

USER 1000
WORKDIR /

# 复制二进制、前端文件等
COPY --from=BACK --chown=$USER:$USER /go/src/casdoor/server_${BUILDX_ARCH} ./server
COPY --from=BACK --chown=$USER:$USER /go/src/casdoor/swagger ./swagger
COPY --from=FRONT --chown=$USER:$USER /web/build ./web/build

# 关键：将配置文件复制到 /conf/app.conf（而不是默认的 ./conf/app.conf）
COPY --from=BACK --chown=$USER:$USER /go/src/casdoor/conf/app.conf /conf/app.conf

# 可选：如果程序仍然尝试在当前目录下寻找 logs/ 目录，创建软链接将其指向 /logs
RUN ln -s /logs /go/src/casdoor/logs 2>/dev/null || true

ENTRYPOINT ["/server"]

FROM debian:latest AS ALLINONE
LABEL MAINTAINER="https://casdoor.org/"
ARG TARGETOS
ARG TARGETARCH
ENV BUILDX_ARCH="${TARGETOS:-linux}_${TARGETARCH:-amd64}"

RUN apt update && apt install -y ca-certificates lsof && update-ca-certificates

WORKDIR /
COPY --from=BACK /go/src/casdoor/server_${BUILDX_ARCH} ./server
COPY --from=BACK /go/src/casdoor/swagger ./swagger
COPY --from=BACK /go/src/casdoor/docker-entrypoint.sh /docker-entrypoint.sh
COPY --from=BACK /go/src/casdoor/conf/app.conf ./conf/app.conf
COPY --from=FRONT /web/build ./web/build

ENTRYPOINT ["/bin/bash"]
CMD ["/docker-entrypoint.sh"]