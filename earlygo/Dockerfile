FROM ubuntu:10.04

RUN apt-get update
RUN apt-get -y install curl mercurial

ENV GOROOT /usr/local/go
ENV GOBIN /usr/local/bin

WORKDIR /usr/local

RUN echo bust1 > /tmp/cache-bust
RUN curl --remote-name http://bradfitz-public.s3-website-us-east-1.amazonaws.com/go.tar.gz
RUN tar -zxvf go.tar.gz
WORKDIR /usr/local/go

ENV GOVERS aaa902b78832

RUN hg update -C $GOVERS
RUN echo $GOVERS > /usr/local/go/VERSION

RUN apt-get -y install make gcc ed bison
WORKDIR /usr/local/go/src
RUN ./make.bash

RUN apt-get -y install emacs22-nox
RUN adduser gopher

RUN whoami
USER gopher
ENV HOME /home/gopher
ENTRYPOINT /bin/bash

