package main

import (
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/buger/jsonparser"
	twilio "github.com/carlosdp/twiliogo"
	"github.com/valyala/fasthttp"
)

// OptionsWithHandler is a struct with a mux and shared credentials
type OptionsWithHandler struct {
	Options *options
}

// NewMOptionsWithHandler returns a OptionsWithHandler for http requests
// with shared credentials
func NewMOptionsWithHandler(o *options) OptionsWithHandler {
	return OptionsWithHandler{
		o,
	}
}

// HandleFastHTTP is the router function
func (m OptionsWithHandler) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	switch string(ctx.Path()) {
	case "/":
		m.ping(ctx)
	case "/send":
		m.sendRequest(ctx)
	default:
		ctx.Error("Not found", fasthttp.StatusNotFound)
	}
}

func (m OptionsWithHandler) ping(ctx *fasthttp.RequestCtx) {
	fmt.Fprint(ctx, "ping")
}

func (m OptionsWithHandler) sendRequest(ctx *fasthttp.RequestCtx) {
	if ctx.IsPost() == false {
		ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
	} else {
		if string(ctx.Request.Header.Peek("Content-Type")) != "application/json" {
			ctx.SetStatusCode(fasthttp.StatusNotAcceptable)
		} else {
			body := ctx.PostBody()
			status, _, _, _ := jsonparser.Get(body, "status")

			if string(status) == "firing" {
				alerts, _, _, _ := jsonparser.Get(body, "alerts")
				jsonparser.ArrayEach(alerts, func(alert []byte, dataType int, offset int, err error) {
					go sendMessage(m.Options, alert)
				})
			}
		}
	}
}

func sendMessage(o *options, alert []byte) {
	c := twilio.NewClient(o.AccountSid, o.AuthToken)
	summary, _, _, _ := jsonparser.Get(alert, "annotations", "summary")

	if string(summary) != "" {
		body := string(summary)

		startsAt, _, _, _ := jsonparser.Get(alert, "startsAt")
		parsedStartsAt, err := time.Parse(time.RFC3339, string(startsAt))
		if err == nil {
			body = "\"" + body + "\"" + " alert starts at " + parsedStartsAt.Format(time.RFC1123)
		}

		message, err := twilio.NewMessage(c, o.Sender, o.Receiver, twilio.Body(body))
		if err != nil {
			log.Error(err)
		} else {
			log.Infof("Message %s", message.Status)
		}
	} else {
		log.Error("Bad format")
	}
}
