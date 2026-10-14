package monitoring

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEvaluateTransition(t *testing.T) {
	var state *State

	steps := []struct {
		up    bool
		event string
	}{
		{true, ""},
		{false, ""},
		{false, "down"},
		{false, ""},
		{false, ""},
		{true, "up"},
		{true, ""},
		{false, ""},
		{true, ""},
	}

	for i, step := range steps {
		next, event := evaluateTransition(state, step.up, 2)
		if event != step.event {
			t.Errorf("step %d: got event %q want %q", i, event, step.event)
		}
		state = &next
	}
}

func TestEvaluateTransition_FirstCheckDown(t *testing.T) {
	state, event := evaluateTransition(nil, false, 1)
	if event != "down" || state.Up {
		t.Errorf("threshold 1 should alert right away, got %q %+v", event, state)
	}
}

func TestSend(t *testing.T) {
	var gotBody string
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotHeaders = r.Header
	}))
	defer server.Close()

	event := Event{Type: "down", AppName: "jellyfin", Error: "http 502"}

	ntfy := Channel{Type: "ntfy", Config: map[string]string{"url": server.URL, "token": "abc"}}
	if err := ntfy.Send(event); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, "jellyfin is down: http 502") || gotHeaders.Get("Authorization") != "Bearer abc" || gotHeaders.Get("Priority") != "high" {
		t.Errorf("ntfy got body %q headers %v", gotBody, gotHeaders)
	}

	discord := Channel{Type: "discord", Config: map[string]string{"url": server.URL}}
	if err := discord.Send(event); err != nil {
		t.Fatal(err)
	}
	var payload map[string]string
	json.Unmarshal([]byte(gotBody), &payload)
	if !strings.Contains(payload["content"], "jellyfin") {
		t.Errorf("discord got %q", gotBody)
	}

	webhook := Channel{Type: "webhook", Config: map[string]string{"url": server.URL, "header": "X-Key", "value": "secret"}}
	if err := webhook.Send(event); err != nil {
		t.Fatal(err)
	}
	if gotHeaders.Get("X-Key") != "secret" || !strings.Contains(gotBody, `"type":"down"`) {
		t.Errorf("webhook got body %q headers %v", gotBody, gotHeaders)
	}
}

func TestMasked(t *testing.T) {
	ch := Channel{Type: "telegram", Config: map[string]string{"token": "123:abc", "chatId": "42"}}
	masked := ch.Masked()
	if masked.Config["token"] != mask || masked.Config["chatId"] != "42" {
		t.Errorf("got %+v", masked.Config)
	}
	if ch.Config["token"] != "123:abc" {
		t.Error("masking changed the original")
	}
}
