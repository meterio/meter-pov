# Build thor in a stock Go builder container
FROM ubuntu:18.04 as builder
RUN apt-get update && apt-get install -y software-properties-common
RUN add-apt-repository ppa:gophers/archive
RUN apt-get update && apt-get -y install build-essential libgmp3-dev git curl golang-1.11-go go-dep 

ENV PATH /usr/lib/go-1.11/bin:$PATH

RUN mkdir -p /go/
ENV GOPATH /go/

WORKDIR  /thor

COPY . .

RUN make dep
RUN go get github.com/ethereum/go-ethereum
RUN cp -r "${GOPATH}/src/github.com/ethereum/go-ethereum/crypto/secp256k1/libsecp256k1" "/thor/vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/"
RUN make thor

# Pull thor into a second stage deploy alpine container
FROM ubuntu:18.04

# RUN apk add --no-cache ca-certificates
COPY --from=builder /thor/bin/thor /usr/local/bin/
COPY --from=builder /thor/crypto/multi_sig/libpbc.so* /usr/local/lib/
ENV LD_LIBRARY_PATH=/usr/local/lib

EXPOSE 8669 11223 11223/udp 11235 11235/udp 5555 8668
ENTRYPOINT ["thor"]
