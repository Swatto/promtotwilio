package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/buger/jsonparser"
	twilio "github.com/carlosdp/twiliogo"
	log "github.com/sirupsen/logrus"
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
	if !ctx.IsPost() {
		ctx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
	} else {
		if string(ctx.Request.Header.Peek("Content-Type")) != "application/json" {
			ctx.SetStatusCode(fasthttp.StatusNotAcceptable)
		} else {
			body := ctx.PostBody()
			status, _ := jsonparser.GetString(body, "status")

			sendOptions := new(options)
			*sendOptions = *m.Options
			const rcvKey = "receiver"
			args := ctx.QueryArgs()
			if nil != args && args.Has(rcvKey) {
				rcv := string(args.Peek(rcvKey))
				sendOptions.Receiver = rcv
			}

			if sendOptions.Receiver == "" {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				log.Error("Bad request: receiver not specified")
				return
			}

			if status == "firing" {
				_, err := jsonparser.ArrayEach(body, func(alert []byte, dataType jsonparser.ValueType, offset int, err error) {

					receivers := strings.Split(sendOptions.Receiver, ",")
					for _, r := range receivers {
						so := new(options)
						so.AccountSid = sendOptions.AccountSid
						so.AuthToken  = sendOptions.AuthToken
						so.Sender     = sendOptions.Sender
						so.Receiver   = r
						log.Infof("Receiver: %v", so.Receiver)
						go sendMessage(so, alert)
					}


				}, "alerts")
				if err != nil {
					log.Warnf("Error parsing json: %v", err)
				}
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
