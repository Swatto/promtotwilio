# Prometheus alert with text message

> !!! WIP !!!

This is a simple and stupid program that will receive webhooks from [Prometheus](https://prometheus.io/) to send a text message (using [Twilio](https://www.twilio.com/)) with the summary of the alert.

The [docker image](https://hub.docker.com/r/swatto/promtotwilio/) size is less than 15Mo.

## Configuration

It needs 4 environment variables:

- `SID` - Twilio Account SID
- `TOKEN` - Twilio Auth Token
- `RECEIVER` - Phone number of receiver
- `SENDER` - Phone number managed by Twilio (friendly name)

You can see a basic launch inside the Makefile.

## Test it

```bash
$ curl -H "Content-Type: application/json" -X POST -d \
'{"version":"2","status":"firing","alerts":[{"annotations":{"summary":"Server down"},"startsAt":"2016-03-19T05:54:01Z"}]}' \
http://local.docker:9090/send
```
