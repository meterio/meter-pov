FROM ubuntu:22.04

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get -y update && apt-get install -y software-properties-common && rm -rf /var/lib/apt/lists/*
RUN add-apt-repository -y ppa:deadsnakes/ppa 
RUN apt-get update && apt-get install -y \
    python3.9 \
    python3.9-distutils \
    python3-pip \
    python3-setuptools \
    python3-wheel \
    supervisor \
    rsyslog \
    rsyslog-relp \
    vim-tiny \
    libgssapi-krb5-2 \
    && rm -rf /var/lib/apt/lists/*
RUN python3.9 -m pip install --no-cache-dir meter-gear==1.2.92