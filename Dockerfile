###
FROM ubuntu:22.04

ENV DEBIAN_FRONTEND noninteractive

RUN apt-get update && apt-get install -y gnupg2

RUN apt-get update \
  && apt-get -y install --no-install-recommends \
    ca-certificates \
    curl \
    dnsutils \
    file \
    ssh \
    sudo \
    wget \
    libjsoncpp-dev libgrpc++ \
    software-properties-common

RUN apt-get update \
  && apt-get install -y fuse3 \
  && echo 'user_allow_other' | tee -a /etc/fuse.conf

# # golang
RUN curl -fsSL https://golang.org/dl/go1.20.1.linux-amd64.tar.gz | tar -xz -C /usr/local
# RUN curl -fsSL https://golang.org/dl/go1.20.1.linux-arm64.tar.gz | tar -xz -C /usr/local

# https://github.com/golang/go/wiki/Ubuntu
# RUN add-apt-repository ppa:longsleep/golang-backports \
#   && apt-get update \
#   && apt-get install -y golang-go

ENV PATH=/usr/local/go/bin:${PATH}

RUN echo "Creating user" \
  && addgroup --gid 1000 ubuntu \
  && adduser --disabled-password --gecos '' --uid 1000 --gid 1000 ubuntu \
  && adduser ubuntu sudo \
  && echo '%sudo ALL=(ALL) NOPASSWD:ALL' | tee -a /etc/sudoers

RUN mkdir -p /mnt \
  && chmod a+rw /mnt

#
RUN mkdir /app

WORKDIR /app

COPY ./go.mod .
COPY ./go.sum .

RUN go mod download

COPY . /app

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

RUN mv /app/nomad /usr/local/bin/nomad

WORKDIR /app

CMD ["/bin/bash"]
