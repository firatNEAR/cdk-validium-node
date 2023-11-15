FROM ghcr.io/near/rollup-data-availability/da-rpc:latest as rust

RUN ls /lib
RUN ls /gopkg/da-rpc

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

COPY . /src

WORKDIR /src

# COPY DA RPC
COPY --from=rust /gopkg/da-rpc/lib/ /usr/local/lib

# BUILD BINARY
RUN cd db && packr2
RUN make build

# CONTAINER FOR RUNNING BINARY
FROM debian:bookworm-slim

COPY --from=build /src/dist/cdk-validium-node /app/cdk-validium-node
COPY --from=rust /gopkg/da-rpc/lib/ /usr/local/lib

RUN apt-get update && apt-get install -y \
    postgresql-client-15 \
    openssl \
    libssl-dev \
    ca-certificates \
    protobuf-compiler

ENV LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH

RUN /app/cdk-validium-node version

EXPOSE 8123
CMD ["/bin/sh", "-c", "/app/cdk-validium-node run"]
