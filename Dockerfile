FROM alpine:3.3

RUN apk add --update ca-certificates && \
    rm -rf /var/cache/apk/* /tmp/*
EXPOSE 9090

COPY promtotwilio /bin
ENTRYPOINT ["/bin/promtotwilio"]
