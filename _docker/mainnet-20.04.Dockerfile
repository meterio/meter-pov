ARG UBUNTU_VERSION=20.04
FROM meterio/mainnet-pos:$UBUNTU_VERSION AS pos
FROM meterio/mainnet-pow:$UBUNTU_VERSION AS pow
FROM meterio/bitcoind-exporter:latest as be

FROM ubuntu:$UBUNTU_VERSION

ARG DEBIAN_FRONTEND=noninteractive

# install necessary packages
RUN apt-get update && apt-get install -y \
  python3-pip \
  python3-setuptools \
  python3-wheel \
  supervisor \
  rsyslog \
  rsyslog-relp \
  vim-tiny \
  libgssapi-krb5-2 \
  && rm -rf /var/lib/apt/lists/*
RUN python3 -m pip install --no-cache-dir meter-gear==1.2.59

# copy PoS binary
COPY --from=pos /usr/bin/meter /usr/bin/
COPY --from=pos /usr/bin/disco /usr/bin/

# copy PoS dependencies
COPY --from=pos /usr/lib/libpbc.so* /usr/lib/
ENV LD_LIBRARY_PATH=/usr/lib:/usr/local/lib

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

EXPOSE 8668 8669 8670 11235 11235/udp 55555/udp 8332 9209 8545 8333
ENTRYPOINT [ "/usr/bin/supervisord" ]
