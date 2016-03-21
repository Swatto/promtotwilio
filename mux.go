package main

import (
	"encoding/json"
	"net/http"

	log "github.com/Sirupsen/logrus"
	twilio "github.com/carlosdp/twiliogo"
)

// MuxWithOptions is a struct with a mux and shared credentials
type MuxWithOptions struct {
	Options *options
	Mux     *http.ServeMux
}

// NewMuxWithOptions returns a MuxWithOptions for http requests
// with shared credentials
func NewMuxWithOptions(o *options) MuxWithOptions {
	mux := http.NewServeMux()
	m := MuxWithOptions{
		o,
		mux,
	}

	m.setup()
	return m
}

func (m MuxWithOptions) setup() {
	m.Mux.Handle("/", http.HandlerFunc(m.ping))
	m.Mux.Handle("/send", http.HandlerFunc(m.sendRequest))
}

func (m MuxWithOptions) ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Ping"))
}

func (m MuxWithOptions) sendRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		code := http.StatusMethodNotAllowed
		w.WriteHeader(code)
		w.Write([]byte(http.StatusText(code)))
	} else {
		decoder := json.NewDecoder(r.Body)
		var data hookData

		err := decoder.Decode(&data)
		if err != nil {
			log.Error(err)
		}

		go sendMessage(m.Options, &data)
	}
}

func sendMessage(o *options, d *hookData) {
	if d.Status == "firing" {
		c := twilio.NewClient(o.AccountSid, o.AuthToken)

		if (len(d.Alerts) > 0) && (d.Alerts[0].Annotations["summary"] != "") {
			body := d.Alerts[0].Annotations["summary"]
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
}
