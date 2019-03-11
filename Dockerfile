FROM golang:1.12-alpine3.9 as builder

RUN mkdir /user && \
    echo 'nobody:x:65534:65534:nobody:/:' > /user/passwd && \
    echo 'nobody:x:65534:' > /user/group

WORKDIR /src
RUN apk add --update --no-cache ca-certificates git

COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 go build \
    -installsuffix 'static' \
    -o /promtotwilio .

FROM scratch

COPY --from=builder /user/group /user/passwd /etc/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /promtotwilio /promtotwilio

EXPOSE 9090
USER nobody:nobody

ENTRYPOINT ["./promtotwilio"]
