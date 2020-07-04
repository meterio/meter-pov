# Build thor in a stock Go builder container
FROM golang:1.11-alpine as builder
RUN apk add --no-cache make gcc musl-dev linux-headers git curl gmp-dev
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

WORKDIR  /thor

COPY . .

RUN cp /thor/crypto/multi_sig/alpine/* /thor/crypto/multi_sig/

RUN make dep
RUN go get github.com/ethereum/go-ethereum
RUN cp -r "${GOPATH}/src/github.com/ethereum/go-ethereum/crypto/secp256k1/libsecp256k1" "/thor/vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/"
RUN make thor

# Pull thor into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates gmp-dev
COPY --from=builder /thor/bin/thor /usr/bin/
COPY --from=builder /thor/crypto/multi_sig/libpbc.* /usr/lib/
ENV LD_LIBRARY_PATH=/usr/lib

EXPOSE 8669 11223 11223/udp 11235 11235/udp 55555 8668
ENTRYPOINT ["thor"]
