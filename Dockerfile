FROM golang:1.9-alpine3.7 as builder

WORKDIR /go/src/github.com/swatto/promtotwilio/
COPY . .

ENV CGO_ENABLED=0
RUN go build -o promtotwilio .

FROM alpine:3.7

EXPOSE 9090
RUN apk add --update --no-cache ca-certificates
WORKDIR /root/

COPY --from=builder /go/src/github.com/swatto/promtotwilio/promtotwilio .
ENTRYPOINT ["./promtotwilio"]
