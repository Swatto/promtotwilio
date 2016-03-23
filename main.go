package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/valyala/fasthttp"
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

	o := NewMOptionsWithHandler(&opts)
	err := fasthttp.ListenAndServe(":9090", o.HandleFastHTTP)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
