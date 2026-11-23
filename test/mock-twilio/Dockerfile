FROM golang:1.27.4-alpine3.24@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS builder

WORKDIR /app
COPY go.mod main.go ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /mock-twilio .

FROM scratch

COPY --from=builder /mock-twilio /mock-twilio

EXPOSE 8080

ENTRYPOINT ["/mock-twilio"]
