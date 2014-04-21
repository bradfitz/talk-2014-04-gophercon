FROM ubuntu:10.04

RUN apt-get update
RUN apt-get -y install curl unzip
RUN apt-get -y install mercurial

ENV GOVERS aaa902b78832

ENV GOPATH /usr/local/go
RUN cd /usr/local && hg clone -u $GOVERS http://code.google.com/p/go

RUN echo $GOVERS > /usr/local/go/VERSION

RUN apt-get -y install make gcc ed
RUN apt-get -y install bison

ENV GOBIN /usr/local/bin

RUN cd $GOPATH/src && ./make.bash

RUN apt-get -y install emacs22-nox
RUN adduser gopher

RUN whoami
USER gopher
ENV HOME /home/gopher
ENTRYPOINT /bin/bash


