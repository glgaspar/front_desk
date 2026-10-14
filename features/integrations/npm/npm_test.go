package npm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFindHost(t *testing.T) {
	hosts := []ProxyHost{
		{Id: 1, DomainNames: []string{"a.example.com"}},
		{Id: 2, DomainNames: []string{"b.example.com", "www.b.example.com"}},
	}
	if h := findHost(hosts, "WWW.b.example.com"); h == nil || h.Id != 2 {
		t.Errorf("got %+v", h)
	}
	if h := findHost(hosts, "c.example.com"); h != nil {
		t.Errorf("should not find c, got %+v", h)
	}
}

func TestHostPayload(t *testing.T) {
	p := hostPayload("app.example.com", "192.168.0.10", 8080, 3)
	if p["certificate_id"] != 3 || p["ssl_forced"] != true || p["forward_port"] != 8080 || p["forward_host"] != "192.168.0.10" {
		t.Errorf("got %+v", p)
	}

	p = hostPayload("app.example.com", "192.168.0.10", 8080, 0)
	if p["ssl_forced"] != false || p["http2_support"] != false {
		t.Errorf("no cert should mean no ssl, got %+v", p)
	}
}

func TestFlexBool(t *testing.T) {
	var host ProxyHost
	json.Unmarshal([]byte(`{"id":1,"ssl_forced":1,"enabled":true}`), &host)
	if !host.SslForced || !host.Enabled {
		t.Errorf("got %+v", host)
	}
	json.Unmarshal([]byte(`{"id":1,"ssl_forced":false,"enabled":0}`), &host)
	if host.SslForced || host.Enabled {
		t.Errorf("got %+v", host)
	}
}

func TestCallWithToken(t *testing.T) {
	logins := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/tokens":
			logins++
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), "admin@example.com") {
				w.WriteHeader(401)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"token": "abc", "expires": time.Now().Add(time.Hour)})
		case r.Header.Get("Authorization") != "Bearer abc":
			w.WriteHeader(401)
		case r.URL.Path == "/api/nginx/proxy-hosts" && r.Method == "GET":
			w.Write([]byte(`[{"id":7,"domain_names":["x.example.com"],"forward_host":"10.0.0.2","forward_port":3000,"certificate_id":0,"ssl_forced":0}]`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	forgetToken()

	c := Config{Url: server.URL, Email: "admin@example.com", Password: "changeme"}
	hosts, err := c.proxyHosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].Id != 7 || hosts[0].ForwardPort != 3000 {
		t.Errorf("got %+v", hosts)
	}

	_, err = c.proxyHosts()
	if err != nil {
		t.Fatal(err)
	}
	if logins != 1 {
		t.Errorf("token should be reused, logged in %d times", logins)
	}

	err = c.call("DELETE", "/api/nginx/proxy-hosts/99", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "status: 404") {
		t.Errorf("expected a 404, got %v", err)
	}
}

func TestCheckCertificate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			json.NewEncoder(w).Encode(map[string]interface{}{"token": "abc", "expires": time.Now().Add(time.Hour)})
			return
		}
		w.Write([]byte(`[{"id":3,"nice_name":"*.example.com","provider":"letsencrypt","domain_names":["*.example.com"]}]`))
	}))
	defer server.Close()
	forgetToken()

	c := Config{Url: server.URL, Email: "a", Password: "b"}
	if err := c.checkCertificate(0); err != nil {
		t.Errorf("0 means no certificate, should pass: %v", err)
	}
	if err := c.checkCertificate(3); err != nil {
		t.Errorf("3 exists: %v", err)
	}
	if err := c.checkCertificate(999); err == nil {
		t.Error("999 does not exist and passed")
	}
}
