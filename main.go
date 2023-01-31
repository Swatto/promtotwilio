package main

import (
	"os"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type options struct {
	AccountSid string
	AuthToken  string
	Receiver   string
	Sender     string
	Port       string
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9090"
	}

	opts := options{
		AccountSid: os.Getenv("SID"),
		AuthToken:  os.Getenv("TOKEN"),
		Receiver:   os.Getenv("RECEIVER"),
		Sender:     os.Getenv("SENDER"),
		Port:       port,
	}

	if opts.AccountSid == "" || opts.AuthToken == "" || opts.Sender == "" {
		log.Fatal("'SID', 'TOKEN' and 'SENDER' environment variables need to be set")
	}

	o := NewMOptionsWithHandler(&opts)
	err := fasthttp.ListenAndServe(fmt.Sprintf(":%s", port), o.HandleFastHTTP)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
