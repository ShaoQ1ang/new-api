ARG GO_IMAGE=golang:1.25-alpine
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS builder

ENV GO111MODULE=on CGO_ENABLED=0
ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
ENV GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB}

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -o /out/ali-video-mock ./cmd/ali-video-mock

FROM scratch

COPY --from=builder /out/ali-video-mock /usr/local/bin/ali-video-mock

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/ali-video-mock"]
