package main

type options struct {
	AccountSid string
	AuthToken  string
	Receiver   string
	Sender     string
}

type alertData struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type hookData struct {
	Version string      `json:"version"`
	Status  string      `json:"status"`
	Alerts  []alertData `json:"alerts"`
}
