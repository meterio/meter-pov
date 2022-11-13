# Build meter in a stock Go builder container
FROM meterio/build-env:22.04 as builder
RUN go version

WORKDIR  /meter

COPY . .

RUN make all

# Pull meter into a second stage deploy alpine container
FROM ubuntu:22.04

# RUN apk add --no-cache ca-certificates
COPY --from=builder /meter/bin/mdb /usr/bin/
COPY --from=builder /meter/bin/meter /usr/bin/
COPY --from=builder /meter/bin/disco /usr/bin/
COPY --from=builder /meter/crypto/multi_sig/libpbc.so* /usr/lib/
ENV LD_LIBRARY_PATH=/usr/lib

EXPOSE 8669 11235 11235/udp 55555/udp 8668 8670
ENTRYPOINT ["meter"]
