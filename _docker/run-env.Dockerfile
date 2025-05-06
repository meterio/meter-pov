FROM ubuntu:24.04

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get -y update && apt-get install -y python3-pip supervisor rsyslog vim-tiny telnet curl && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir meter-gear==1.2.92 --break-system-packages