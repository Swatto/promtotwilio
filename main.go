package main

import (
	"encoding/json"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	twilio "github.com/carlosdp/twiliogo"
)

var (
	opts options
)

func main() {
	opts = options{
		AccountSid: os.Getenv("SID"),
		AuthToken:  os.Getenv("TOKEN"),
		Receiver:   os.Getenv("RECEIVER"),
		Sender:     os.Getenv("SENDER"),
	}

	if opts.AccountSid == "" || opts.AuthToken == "" || opts.Receiver == "" || opts.Sender == "" {
		log.Fatal("'SID', 'TOKEN', 'RECEIVER' and 'SENDER' environment variables need to be set")
	}

	http.HandleFunc("/", healthRequest)
	http.HandleFunc("/send", sendRequest)
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func sendRequest(w http.ResponseWriter, r *http.Request) {
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

		if data.Status == "firing" {
			go sendMessage(&data)
		}
	}
}

func healthRequest(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("Ping"))
}

func sendMessage(data *hookData) {
	c := twilio.NewClient(opts.AccountSid, opts.AuthToken)

	if (len(data.Alerts) > 0) && (data.Alerts[0].Annotations["summary"] != "") {
		body := data.Alerts[0].Annotations["summary"]
		message, err := twilio.NewMessage(c, opts.Sender, opts.Receiver, twilio.Body(body))
		if err != nil {
			log.Error(err)
		} else {
			log.Infof("Message %s", message.Status)
		}
	}
}
