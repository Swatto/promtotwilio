package main

import "testing"

func TestFindAndReplaceLables(t *testing.T) {
	alert := []byte(`
    {
      "status": "firing",
      "labels": {
        "alertname": "InstanceDown",
        "instance": "http://test.com",
        "job": "blackbox"
      },
      "annotations": {
        "description": "Unable to scrape $labels.instance",
        "summary": "Address $labels.instance appears to be down"
      },
      "startsAt": "2017-01-06T19:34:52.887Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://test.com/graph?g0.expr=probe_success%7Bjob%3D%22blackbox%22%7D+%3D%3D+0&g0.tab=0"
    }
  `)

	input := "Address $labels.instance appears to be down with $labels.alertname"
	expected := "Address http://test.com appears to be down with InstanceDown"
	output := findAndReplaceLables(input, alert)

	if output != expected {
		t.Errorf("findAndReplaceLables(%q, alert) == %q, want %q", input, output, expected)
	}
}
