FROM golang:1.24.2 AS build

ARG VERSION=dev
ARG COMMIT_HASH
ENV CGO_ENABLED=0

WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./ ./
RUN go mod download
RUN CGO_ENABLED=${CGO_ENABLED} go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}' -X 'main.GoVersion=$(go version | awk '{print $3}' | sed 's/^go//')'" -o /tunnelguard .


FROM alpine:3.21.3 AS final

LABEL maintainer="soerenschneider"
COPY --from=build /tunnelguard /tunnelguard
RUN apk add --no-cache \
    wireguard-tools \
    linux-headers \
    iproute2 \
    iptables

ENTRYPOINT ["/tunnelguard"]
