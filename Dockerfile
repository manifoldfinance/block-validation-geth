# syntax = docker/dockerfile:1.4
FROM golang:1.18-alpine as builder

RUN apk add --no-cache gcc musl-dev linux-headers git

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /go-ethereum/
COPY go.sum /go-ethereum/
RUN cd /go-ethereum && go mod download

ADD . /go-ethereum
RUN --mount=type=cache,target=/root/.cache \
    --mount=type=cache,target=/tmp/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd /go-ethereum && go run build/ci.go install ./cmd/geth

FROM docker.io/library/alpine:3.15

# Pull Geth into a second stage deploy alpine container

RUN apk add --no-cache ca-certificates
COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/
RUN addgroup -g 10001 -S geth && adduser -u 10000 -S -G geth -h /home/geth geth
# bind-tools is needed for DNS resolution to work in *some* Docker networks, but not all.
# This applies to nslookup, Go binaries, etc. If you want your Docker image to work even
# in more obscure Docker environments, use this.
RUN apk add --no-cache bind-tools
ENV LANG=en_US.UTF-8

#EXPOSE 8545 8546 30303 30303/udp
EXPOSE 8545 8551 8546 30303 30303/udp 42069 42069/udp 8080 9090 6060
ENTRYPOINT ["geth"]

# https://github.com/opencontainers/image-spec/blob/main/annotations.md
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
ARG COMMIT=""
ARG VERSION=""
ARG BUILDNUM=""
LABEL org.label-schema.build-date=$BUILD_DATE \
      org.label-schema.name="block-validation-geth" \
      org.label-schema.description="Block Validation for Go-Ethereum" \
      org.label-schema.url="https://torquem.ch" \
      org.label-schema.vcs-ref=$COMMIT \
      org.label-schema.vcs-url="https://github.com/manifoldfinance/block-validation-geth.git" \
      org.label-schema.vendor="CommodityStream, Inc." \
      org.label-schema.version=$$BUILDNUM \
      org.label-schema.schema-version="1.0"