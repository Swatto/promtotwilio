FROM golang:1.25-alpine AS builder

RUN mkdir /user && \
    echo 'nobody:x:65534:65534:nobody:/:' > /user/passwd && \
    echo 'nobody:x:65534:' > /user/group

WORKDIR /src
RUN apk add --update --no-cache ca-certificates

COPY ./go.mod ./
# go.sum is not present when there are no external dependencies

COPY ./ ./

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /promtotwilio ./cmd/promtotwilio

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/swatto/promtotwilio"
LABEL org.opencontainers.image.description="Prometheus to Twilio bridge"

COPY --from=builder /user/group /user/passwd /etc/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /promtotwilio /promtotwilio

EXPOSE 9090
USER nobody:nobody

ENTRYPOINT ["/promtotwilio"]
