FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod main.go ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /mock-twilio .

FROM scratch

COPY --from=builder /mock-twilio /mock-twilio

EXPOSE 8080

ENTRYPOINT ["/mock-twilio"]
