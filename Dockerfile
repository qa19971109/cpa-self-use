ARG GO_IMAGE=docker.1ms.run/library/golang:1.26-alpine
ARG ALPINE_IMAGE=golang:1.26-alpine
ARG GOPROXY=https://goproxy.cn,direct
ARG ALPINE_REPO=https://mirrors.aliyun.com/alpine

FROM ${GO_IMAGE} AS builder

WORKDIR /app
ARG GOPROXY
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./CLIProxyAPI ./cmd/server/

FROM ${ALPINE_IMAGE}

ARG ALPINE_REPO
RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_REPO}|g" /etc/apk/repositories && apk add --no-cache tzdata

RUN mkdir /CLIProxyAPI

COPY --from=builder ./app/CLIProxyAPI /CLIProxyAPI/CLIProxyAPI

COPY config.example.yaml /CLIProxyAPI/config.example.yaml

WORKDIR /CLIProxyAPI

EXPOSE 8317

ENV TZ=Asia/Shanghai

RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

CMD ["./CLIProxyAPI"]
