FROM golang:1.23.1 AS build

WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./ ./
ENV CGO_ENABLED=0
RUN go mod download
RUN CGO_ENABLED=0 go build -o /tunnelguard .


FROM alpine:3.20.2 AS final

LABEL maintainer="soerenschneider"
COPY --from=build /tunnelguard /tunnelguard
RUN apk add --no-cache \
    wireguard-tools \
    linux-headers \
    iproute2 \
    iptables

ENTRYPOINT ["/tunnelguard"]
