package apps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseRef(t *testing.T) {
	cases := map[string][3]string{
		"nginx":                   {"registry-1.docker.io", "library/nginx", "latest"},
		"nginx:1.25":              {"registry-1.docker.io", "library/nginx", "1.25"},
		"linuxserver/jellyfin:10": {"registry-1.docker.io", "linuxserver/jellyfin", "10"},
		"docker.io/library/redis": {"registry-1.docker.io", "library/redis", "latest"},
		"ghcr.io/home-assistant/home-assistant:stable": {"ghcr.io", "home-assistant/home-assistant", "stable"},
		"lscr.io/linuxserver/jellyfin":                 {"lscr.io", "linuxserver/jellyfin", "latest"},
		"localhost:5000/x:y":                           {"localhost:5000", "x", "y"},
	}
	for ref, want := range cases {
		registry, repo, tag, err := parseRef(ref)
		if err != nil {
			t.Errorf("%s: %v", ref, err)
			continue
		}
		if registry != want[0] || repo != want[1] || tag != want[2] {
			t.Errorf("%s got %s %s %s", ref, registry, repo, tag)
		}
	}

	if _, _, _, err := parseRef("nginx@sha256:abc"); !errors.Is(err, errPinned) {
		t.Errorf("pinned image should say so, got %v", err)
	}
}

func TestParseAuthenticate(t *testing.T) {
	got := parseAuthenticate(`Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"`)
	if got["realm"] != "https://auth.docker.io/token" || got["service"] != "registry.docker.io" || got["scope"] != "repository:library/nginx:pull" {
		t.Errorf("got %+v", got)
	}
	if len(parseAuthenticate(`Basic realm="x"`)) != 0 {
		t.Error("only bearer is supported")
	}
}

func TestRemoteDigest(t *testing.T) {
	var methods []string
	limited := false
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if r.URL.Query().Get("scope") != "repository:library/nginx:pull" {
				w.WriteHeader(400)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"token": "abc"})
			return
		}
		methods = append(methods, r.Method)
		if limited {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		if r.Header.Get("Authorization") != "Bearer abc" {
			w.Header().Set("Www-Authenticate", `Bearer realm="`+server.URL+`/token",service="test",scope="repository:library/nginx:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.Header.Get("Accept"), "manifest.list.v2") {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Docker-Content-Digest", "sha256:123")
	}))
	defer server.Close()

	old := registryBase
	registryBase = func(host string) string { return server.URL }
	defer func() { registryBase = old }()

	digest, err := remoteDigest(context.Background(), "nginx:1.25")
	if err != nil || digest != "sha256:123" {
		t.Fatalf("got %q %v", digest, err)
	}
	for _, m := range methods {
		if m != "HEAD" {
			t.Errorf("manifest got a %s, every GET is a pull on docker hub", m)
		}
	}

	limited = true
	_, err = remoteDigest(context.Background(), "nginx:1.25")
	if !errors.Is(err, errRateLimited) {
		t.Errorf("expected rate limited, got %v", err)
	}
}
