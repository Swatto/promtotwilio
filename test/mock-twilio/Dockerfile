FROM golang:1.27.4-alpine3.24@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf AS builder

WORKDIR /app
COPY go.mod main.go ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /mock-twilio .

FROM scratch

COPY --from=builder /mock-twilio /mock-twilio

EXPOSE 8080

ENTRYPOINT ["/mock-twilio"]
