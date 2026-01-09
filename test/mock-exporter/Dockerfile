FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY main.go ./

RUN go build -o mock-exporter .

FROM alpine:3.21

COPY --from=builder /app/mock-exporter /mock-exporter

EXPOSE 9100

ENTRYPOINT ["/mock-exporter"]
