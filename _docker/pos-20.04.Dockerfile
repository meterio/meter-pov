# Build meter in a stock Go builder container
ARG UBUNTU_VERSION=20.04
FROM meterio/build-env:$UBUNTU_VERSION as builder
RUN go version

WORKDIR  /meter

COPY . .

#RUN git submodule update --init
# RUN make dep (takes much longer)

# prepare for missed sha3 library
#RUN go get golang.org/x/crypto/sha3
#RUN cp -r "${GOPATH}/src/golang.org/x/crypto/sha3" "/meter/vendor/golang.org/x/crypto/sha3"

# prepare for missed secp256k1 library
# RUN go get github.com/ethereum/go-ethereum
# RUN cp -r "${GOPATH}/src/github.com/ethereum/go-ethereum/crypto/secp256k1/libsecp256k1" "/meter/vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/"

RUN make all

# Pull meter into a second stage deploy alpine container
FROM ubuntu:20.04

# RUN apk add --no-cache ca-certificates
COPY --from=builder /meter/bin/meter /usr/bin/
COPY --from=builder /meter/bin/disco /usr/bin/
COPY --from=builder /meter/crypto/multi_sig/libpbc.so* /usr/lib/
ENV LD_LIBRARY_PATH=/usr/lib

EXPOSE 8669 11235 11235/udp 55555/udp 8668 8670
ENTRYPOINT ["meter"]
