# Prometheus alert with text message

> !!! WIP !!!

This is a simple and stupid program that will listen to webhook of Prometheus to send a text message with the summary of the alert.

## Configuration

It needs 4 environment variables:

- `SID` - Twilio Account SID
- `TOKEN` - Twilio Auth Token
- `RECEIVER` - Phone number of receiver
- `SENDER` - Phone number managed by Twilio (friendly name)
