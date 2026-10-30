package widgets

import (
	"strings"
	"testing"
)

const tdarrManifest = `{
	"fields": {"url": {"label": "Url"}, "apiKey": {"label": "API key", "secret": true, "optional": true}},
	"auth": {"headers": {"x-api-key": "{apiKey}"}},
	"allow": [{"method": "POST", "path": "/api/v2/cruddb"}, {"method": "POST", "path": "/api/v2/kill", "admin": true}]
}`

func TestManifestNeedsUrl(t *testing.T) {
	if _, err := parseManifest(`{"fields": {"key": {"label": "Key"}}}`); err == nil {
		t.Error("a manifest without url should fail")
	}
	m, err := parseManifest(`{"fields": {"url": {"label": "Url"}}}`)
	if err != nil || len(m.Allow) != 1 || m.Allow[0].Method != "GET" {
		t.Errorf("no allow should mean GET only, got %v %+v", err, m)
	}
}

func TestBuildRequest(t *testing.T) {
	m, _ := parseManifest(tdarrManifest)
	values := map[string]string{"url": "http://192.168.0.10:8265", "apiKey": "secret"}

	req, rule, err := buildRequest(m, values, "post", "/api/v2/cruddb", []byte(`{"data":{}}`), false)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "http://192.168.0.10:8265/api/v2/cruddb" || req.Header.Get("x-api-key") != "secret" || rule.Admin {
		t.Errorf("got %s %v %+v", req.URL, req.Header, rule)
	}

	// optional key not set, the header doesnt go at all
	req, _, _ = buildRequest(m, map[string]string{"url": "http://192.168.0.10:8265"}, "POST", "/api/v2/cruddb", nil, false)
	if _, ok := req.Header["X-Api-Key"]; ok {
		t.Errorf("empty key should not be sent, got %v", req.Header)
	}
}

func TestBuildRequestRefuses(t *testing.T) {
	m, _ := parseManifest(tdarrManifest)
	values := map[string]string{"url": "http://192.168.0.10:8265/tdarr"}

	cases := map[string]struct {
		method, path string
		admin        bool
		want         string
	}{
		"not in allow":      {"GET", "/api/v2/cruddb", true, "not allowed"},
		"other path":        {"POST", "/api/v2/other", true, "not allowed"},
		"admin only":        {"POST", "/api/v2/kill", false, "only admins"},
		"going up a folder": {"POST", "/api/v2/cruddb/../../../radarr", true, "cant go up"},
		"no slash":          {"POST", "api/v2/cruddb", true, "must start with /"},
		"backslash":         {"POST", "/api/v2/cruddb\\x", true, "must start with /"},
	}
	for name, c := range cases {
		_, _, err := buildRequest(m, values, c.method, c.path, nil, c.admin)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v", name, err)
		}
	}

	if _, _, err := buildRequest(m, values, "POST", "/api/v2/kill", nil, true); err != nil {
		t.Errorf("admin should be able to, got %v", err)
	}
}

func TestBuildRequestStaysOnTheHost(t *testing.T) {
	m, _ := parseManifest(`{"fields": {"url": {"label": "Url"}}}`)
	values := map[string]string{"url": "http://192.168.0.10:8989"}

	for _, path := range []string{"//evil.com/x", "/@evil.com/x", "/.evil.com"} {
		req, _, err := buildRequest(m, values, "GET", path, nil, false)
		if err == nil && req.URL.Host != "192.168.0.10:8989" {
			t.Errorf("%s left the host: %s", path, req.URL)
		}
	}

	// the base path stays, it doesnt get replaced by the path
	req, _, err := buildRequest(m, map[string]string{"url": "http://box/sonarr/"}, "GET", "/api/v3/queue?page=2", nil, false)
	if err != nil || req.URL.String() != "http://box/sonarr/api/v3/queue?page=2" {
		t.Errorf("got %v %v", req.URL, err)
	}
}

func TestQueryAuthAndBearer(t *testing.T) {
	m, _ := parseManifest(`{"fields": {"url": {"label": "Url"}, "token": {"label": "Token", "secret": true}},
		"auth": {"query": {"apikey": "{token}"}, "headers": {"Authorization": "Bearer {token}"}}}`)
	req, _, err := buildRequest(m, map[string]string{"url": "http://box", "token": "t0k"}, "GET", "/x?a=1", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.Query().Get("apikey") != "t0k" || req.URL.Query().Get("a") != "1" || req.Header.Get("Authorization") != "Bearer t0k" {
		t.Errorf("got %s %v", req.URL, req.Header)
	}
}
