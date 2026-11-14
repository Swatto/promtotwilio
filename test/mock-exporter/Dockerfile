FROM golang:1.27.4-alpine3.24@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf AS builder

WORKDIR /app
COPY go.mod ./
COPY main.go ./

RUN go build -o mock-exporter .

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

COPY --from=builder /app/mock-exporter /mock-exporter

EXPOSE 9100

ENTRYPOINT ["/mock-exporter"]
