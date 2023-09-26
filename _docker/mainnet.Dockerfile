FROM meterio/mainnet-pow:latest AS pow
FROM meterio/bitcoind-exporter:latest as be

# Build PoS with golang 1.19
FROM meterio/build-env:latest as pos
RUN go version
WORKDIR  /meter
COPY . .
RUN make all


FROM meterio/run-env:latest
# copy PoW binary
COPY --from=pow /usr/local/bin/bitcoind /usr/bin/
COPY --from=pow /usr/local/bin/bitcoin-cli /usr/bin/
COPY --from=pow /usr/local/bin/bitcoin-tx /usr/bin/

# copy PoW dependencies
COPY --from=pow /usr/lib/libboost*.so* /usr/lib/
COPY --from=pow /usr/lib/libssl*.so* /usr/lib/
COPY --from=pow /usr/lib/libevent*.so* /usr/lib/
COPY --from=pow /usr/lib/libcrypto*.so* /usr/lib/
COPY --from=pow /usr/lib/libminiupnpc*.so* /usr/lib/
COPY --from=pow /usr/lib/libzmq*.so* /usr/lib/
COPY --from=pow /usr/lib/libstdc++*.so* /usr/lib/
COPY --from=pow /usr/lib/libsodium*.so* /usr/lib/
COPY --from=pow /usr/lib/libpgm*.so* /usr/lib/
COPY --from=pow /usr/lib/libnorm*.so* /usr/lib/

# copy bitcoind-exporter binary
COPY --from=be /usr/bin/bitcoind_exporter /usr/bin/

# config bitcoind-exporter
ENV BTC_USER=testuser
ENV BTC_PASS=testpass
ENV BTC_HOST=127.0.0.1:8332
ENV HTTP_LISTENADDR=:8333

# prepare data dir
RUN mkdir /pos
RUN mkdir /pow

# prepare config files
COPY _docker/main/bitcoin.conf /pow/bitcoin.conf
COPY _docker/main/00-meter.conf /etc/rsyslog.d/
COPY _docker/main/rsyslog.conf /etc/rsyslog.conf
COPY _docker/main/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY _docker/main/reset.sh /

# enable executable for reset script
RUN chmod a+x /reset.sh

LABEL com.centurylinklabs.watchtower.lifecycle.pre-update="/reset.sh"

# set extra env 
ENV POS_EXTRA=
ENV POW_EXTRA=

# create log output file for supervisor
RUN touch /var/log/supervisor/pos.log
RUN touch /var/log/supervisor/pow.log
RUN touch /var/log/supervisor/gear.log
RUN touch /var/log/supervisor/bitcoind_exporter.log



# copy PoS binary
COPY --from=pos /meter/bin/meter /usr/bin/
COPY --from=pos /meter/bin/disco /usr/bin/
COPY --from=pos /meter/bin/mdb /usr/bin/
ENV MDB_NETWORK=main
ENV MDB_DATA_DIR=/pos

# copy PoS dependencies
COPY --from=pos /meter/crypto/multi_sig/libpbc.so* /usr/lib/
ENV LD_LIBRARY_PATH=/usr/lib:/usr/local/lib

EXPOSE 8668 8669 8670 11235 11235/udp 55555/udp 8332 9209 8545 8333
ENTRYPOINT [ "/usr/bin/supervisord" ]
