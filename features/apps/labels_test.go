package apps

import (
	"testing"
)

func TestParseComposeLabels_Map(t *testing.T) {
	compose := `
services:
  web:
    image: nginx
    labels:
      front-desk.name: My App
      front-desk.url: https://app.example.com:8443
      front-desk.dir: myapp
      front-desk.cloudflare-port: 8080
`
	labels, err := ParseComposeLabels(compose)
	if err != nil {
		t.Fatal(err)
	}
	if labels.Name != "My App" || labels.Dir != "/myapp" || labels.Port != "8080" {
		t.Errorf("got %+v", labels)
	}
}

func TestParseComposeLabels_List(t *testing.T) {
	compose := `
services:
  db:
    image: postgres
  web:
    image: nginx
    labels:
      - "front-desk.name=thing"
      - "front-desk.url=http://thing.local:3000"
      - "front-desk.dir=/thing"
`
	labels, err := ParseComposeLabels(compose)
	if err != nil {
		t.Fatal(err)
	}
	if labels.Name != "thing" || labels.Dir != "/thing" || labels.Port != "" {
		t.Errorf("got %+v", labels)
	}
}

func TestParseComposeLabels_Bad(t *testing.T) {
	bad := []string{
		"not: [yaml",
		"services:\n  web:\n    image: nginx\n",
		"services:\n  web:\n    labels:\n      front-desk.name: x\n",
		"services:\n  web:\n    labels:\n      front-desk.name: x\n      front-desk.dir: /\n",
		"services:\n  web:\n    labels:\n      front-desk.dir: /x\n",
	}
	for _, compose := range bad {
		if _, err := ParseComposeLabels(compose); err == nil {
			t.Errorf("should fail:\n%s", compose)
		}
	}
}

func TestParseComposeLabels_Port(t *testing.T) {
	compose := `
services:
  web:
    labels:
      front-desk.name: x
      front-desk.dir: /x
      front-desk.port: "3000"
      front-desk.cloudflare-port: "8080"
`
	labels, err := ParseComposeLabels(compose)
	if err != nil {
		t.Fatal(err)
	}
	if labels.Port != "3000" {
		t.Errorf("front-desk.port should win, got %q", labels.Port)
	}
}
