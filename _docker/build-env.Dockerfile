FROM ubuntu:24.04

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get -y update && apt-get install -y git make build-essential libgmp-dev vim-tiny telnet curl wget tar dnsutils libdb++-dev libssl-dev libevent-dev pkg-config make build-essential bsdmainutils libboost1.74-all-dev && rm -rf /var/lib/apt/lists/*
RUN wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz -O go.tar.gz
RUN tar -xzvf go.tar.gz -C /usr/local
RUN mv /usr/local/go/bin/go /usr/local/bin/go
RUN mv /usr/local/go/bin/gofmt /usr/local/bin/gofmt

ENV GOROOT=/usr/local/go