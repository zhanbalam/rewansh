FROM --platform=${BUILDPLATFORM} golang:1.18-alpine as builder

RUN apk update && rm -rf /var/lib/apt/lists/* /var/cache/apk/*

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

ARG TARGETOS
ARG TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -a -o rewansh .

FROM alpine

COPY --from=builder /app/rewansh /bin/rewansh

ENTRYPOINT ["/bin/rewansh", "-c", "/etc/rewansh/config.yaml"]
