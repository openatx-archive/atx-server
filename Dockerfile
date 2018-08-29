FROM golang:1.10
RUN go get -v github.com/openatx/atx-server && cd $GOPATH/src/github.com/openatx/atx-server && go build

FROM debian:stretch
WORKDIR /root/
COPY --from=0 /go/src/github.com/openatx/atx-server .
ENTRYPOINT ./atx-server --port 8000
