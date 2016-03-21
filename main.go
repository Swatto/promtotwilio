package main

import (
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
)

func main() {
	opts := options{
		AccountSid: os.Getenv("SID"),
		AuthToken:  os.Getenv("TOKEN"),
		Receiver:   os.Getenv("RECEIVER"),
		Sender:     os.Getenv("SENDER"),
	}

	if opts.AccountSid == "" || opts.AuthToken == "" || opts.Receiver == "" || opts.Sender == "" {
		log.Fatal("'SID', 'TOKEN', 'RECEIVER' and 'SENDER' environment variables need to be set")
	}

	m := NewMuxWithOptions(&opts)

	err := http.ListenAndServe(":9090", m.Mux)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
