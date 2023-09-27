FROM us-docker.pkg.dev/pagoda-solutions-dev/rollup-data-availability/op-rpc:0.0.1-bullseye as rust

RUN ls /app/lib

# CONTAINER FOR BUILDING BINARY
FROM golang:1.19 AS build

# INSTALL DEPENDENCIES
RUN go install github.com/gobuffalo/packr/v2/packr2@v2.8.3
COPY go.mod go.sum /src/
RUN cd /src && go mod download

RUN apt-get update && apt-get install -y \
    openssl \
    libssl-dev \
    protobuf-compiler

COPY ./da-rpc /op-stack/da-rpc
RUN mkdir -p /op-stack/da-rpc/lib
COPY --from=rust /app/lib/libnear-da-op-rpc-sys.h /op-stack/da-rpc/lib/
COPY --from=rust /app/lib/libnear_da_op_rpc_sys.so /op-stack/da-rpc/lib/
RUN cat /op-stack/da-rpc/lib/libnear-da-op-rpc-sys.h
RUN ls /op-stack/da-rpc/lib -l

# BUILD BINARY
COPY . /src
RUN cd /src/db && packr2
RUN cd /src && make build

# CONTAINER FOR RUNNING BINARY
# TODO: urgh FROM alpine:3.18.0
FROM debian:bookworm-slim

COPY --from=build /src/dist/cdk-validium-node /app/cdk-validium-node
COPY --from=build /op-stack/da-rpc/lib /usr/local/lib/

RUN apt-get update && apt-get install -y \
    postgresql-client-15 \
    openssl \
    libssl-dev \
    ca-certificates \
    protobuf-compiler
#RUN apk update && apk add postgresql15-client

ENV LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH

RUN /app/cdk-validium-node version

EXPOSE 8123
CMD ["/bin/sh", "-c", "/app/cdk-validium-node run"]
