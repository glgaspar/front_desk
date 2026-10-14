package cloudflare

import (
	"testing"
)

func host(h string) *string { return &h }

func TestInsertIngress_KeepsCatchAllLast(t *testing.T) {
	rules := []IngressRule{
		{Hostname: host("a.example.com"), Service: "http://localhost:1"},
		{Service: "http_status:404"},
	}

	rules = insertIngress(rules, IngressRule{Hostname: host("b.example.com"), Service: "http://localhost:2"})
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[len(rules)-1].Hostname != nil {
		t.Error("catch all is not last anymore")
	}

	again := insertIngress(rules, IngressRule{Hostname: host("b.example.com"), Service: "http://localhost:2"})
	if len(again) != 3 {
		t.Error("inserting the same hostname twice should do nothing")
	}
}

func TestFilterIngress(t *testing.T) {
	rules := []IngressRule{
		{Hostname: host("a.example.com"), Service: "http://localhost:1"},
		{Hostname: host("b.example.com"), Service: "http://localhost:2"},
		{Service: "http_status:404"},
	}

	filtered := filterIngress(rules, "a.example.com")
	if len(filtered) != 2 || *filtered[0].Hostname != "b.example.com" {
		t.Errorf("got %+v", filtered)
	}
	if filtered[len(filtered)-1].Hostname != nil {
		t.Error("catch all got removed")
	}

	if len(filterIngress(rules, "")) != 3 {
		t.Error("empty hostname should not remove the catch all")
	}
}

func TestHostFromUrl(t *testing.T) {
	cases := map[string]string{
		"https://app.example.com":       "app.example.com",
		"app.example.com":               "app.example.com",
		"http://app.example.com:8080/x": "app.example.com",
		"":                              "",
	}
	for in, want := range cases {
		if got := HostFromUrl(in); got != want {
			t.Errorf("%q got %q want %q", in, got, want)
		}
	}
}
