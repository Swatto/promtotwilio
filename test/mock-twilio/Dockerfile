FROM golang:1.27.3-alpine3.24@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS builder

WORKDIR /app
COPY go.mod main.go ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /mock-twilio .

FROM scratch

COPY --from=builder /mock-twilio /mock-twilio

EXPOSE 8080

ENTRYPOINT ["/mock-twilio"]
