FROM meterio/pos:mainnet AS pos
FROM meterio/pow:mainnet AS pow
FROM meterio/bitcoind-exporter:latest as be

FROM ubuntu:18.04

# necessary packages
RUN apt-get update 
RUN apt-get install -y --no-install-recommends supervisor rsyslog rsyslog-relp vim-tiny && apt-get clean 
RUN apt-get install -y --no-install-recommends build-essential gcc python3-minimal python3-dev python3-pip python3-setuptools python3-wheel && pip3 install --no-cache-dir meter-gear==1.0.14 && apt-get remove -y gcc python3-dev build-essential && apt-get clean

# POS settings 
COPY --from=pos /usr/bin/meter /usr/bin/
COPY --from=pos /usr/bin/disco /usr/bin/
COPY --from=pos /usr/lib/libpbc.so* /usr/lib/
ENV LD_LIBRARY_PATH=/usr/lib:/usr/local/lib

# POW settings
COPY --from=pow /usr/local/bin/bitcoind /usr/bin/
COPY --from=pow /usr/local/bin/bitcoin-cli /usr/bin/
COPY --from=pow /usr/local/bin/bitcoin-tx /usr/bin/

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
COPY --from=pow /usr/lib/libdb*.so* /usr/lib/

COPY --from=be /usr/bin/bitcoind_exporter /usr/bin/


ENV POS_EXTRA=
ENV POW_EXTRA=

# bitcoind-exporter settings
ENV BTC_USER=testuser
ENV BTC_PASS=testpass
ENV BTC_HOST=127.0.0.1:8332
ENV HTTP_LISTENADDR=:8333

# meter-gear settings
RUN cp /usr/lib/x86_64-linux-gnu/libcrypto.so.1.1 /usr/lib/
RUN cp /usr/lib/x86_64-linux-gnu/libssl.so.1.1 /usr/lib/
ENV LC_ALL=C.UTF-8
ENV LANG=C.UTF-8


RUN mkdir /pow
RUN mkdir /pos

COPY _docker/main/bitcoin.conf /pow/bitcoin.conf
COPY _docker/main/00-meter.conf /etc/rsyslog.d/
COPY _docker/main/rsyslog.conf /etc/rsyslog.conf
COPY _docker/main/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY _docker/main/reset.sh /

RUN chmod a+x /reset.sh

RUN touch /var/log/supervisor/pos-stdout.log
RUN touch /var/log/supervisor/pos-stderr.log

LABEL com.centurylinklabs.watchtower.lifecycle.pre-update="/reset.sh"

EXPOSE 8668 8669 8670 11235 11235/udp 55555/udp 8332 9209 8545 8333
ENTRYPOINT [ "/usr/bin/supervisord" ]

