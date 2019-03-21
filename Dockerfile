# Build thor in a stock Go builder container
FROM dfinlab/pos-build as builder

WORKDIR  /thor

COPY . .

RUN make dep
RUN go get github.com/ethereum/go-ethereum
RUN cp -r "${GOPATH}/src/github.com/ethereum/go-ethereum/crypto/secp256k1/libsecp256k1" "/thor/vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/"
RUN make thor
RUN make disco

# Pull thor into a second stage deploy alpine container
FROM ubuntu:18.04

# RUN apk add --no-cache ca-certificates
COPY --from=builder /thor/bin/thor /usr/bin/
COPY --from=builder /thor/bin/disco /usr/bin/
COPY --from=builder /thor/crypto/multi_sig/libpbc.so* /usr/lib/
ENV LD_LIBRARY_PATH=/usr/lib

EXPOSE 8669 11223 11223/udp 11235 11235/udp 55555 8668
ENTRYPOINT ["thor"]
