FROM alpine:3.20

RUN apk add --no-cache curl jq bash

COPY run-tests.sh /run-tests.sh
RUN chmod +x /run-tests.sh

ENTRYPOINT ["/run-tests.sh"]
