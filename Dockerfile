FROM golang:1.11
RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go build

FROM debian:stretch
WORKDIR /root/
COPY --from=0 /app/atx-server .
COPY --from=0 /app/templates ./templates
ENTRYPOINT ./atx-server --port 8000
