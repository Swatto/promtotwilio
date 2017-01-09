package main

import (
	"fmt"
	"regexp"
	"strings"
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
			status, _ := jsonparser.GetString(body, "status")

			if status == "firing" {
				jsonparser.ArrayEach(body, func(alert []byte, dataType jsonparser.ValueType, offset int, err error) {
					go sendMessage(m.Options, alert)
				}, "alerts")
			}
		}
	}
}

func sendMessage(o *options, alert []byte) {
	c := twilio.NewClient(o.AccountSid, o.AuthToken)
	body, _ := jsonparser.GetString(alert, "annotations", "summary")

	if body != "" {
		body = findAndReplaceLables(body, alert)
		startsAt, _ := jsonparser.GetString(alert, "startsAt")
		parsedStartsAt, err := time.Parse(time.RFC3339, startsAt)
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

func findAndReplaceLables(body string, alert []byte) string {
	labelReg := regexp.MustCompile(`\$labels.[a-z]+`)
	matches := labelReg.FindAllString(body, -1)

	if matches != nil {
		for _, match := range matches {
			labelName := strings.Split(match, ".")
			if len(labelName) == 2 {
				replaceWith, _ := jsonparser.GetString(alert, "labels", labelName[1])
				body = strings.Replace(body, match, replaceWith, -1)
			}
		}
	}

	return body
}
