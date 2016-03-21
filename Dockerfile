FROM scratch

EXPOSE 9090

COPY promtotwilio /
ENTRYPOINT ["/promtotwilio"]
