FROM golang:1.11
RUN go build

FROM debian:stretch
WORKDIR /root/
COPY --from=0 /go/src/github.com/openatx/atx-server .
ENTRYPOINT ./atx-server --port 8000
