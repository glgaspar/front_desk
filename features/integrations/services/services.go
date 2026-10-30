package services

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/glgaspar/front_desk/features/integrations"
)

// most of the homelab apps only need an url and a key, so they all share one table.
// extra is whatever else the service needs (ollama model, pihole lan ip)
type Config struct {
	Url      string `json:"url"`
	Username string `json:"username"`
	Secret   string `json:"secret"`
	Extra    string `json:"extra"`
}

type service struct {
	needsUrl    bool
	needsSecret bool
	check       func(Config) error
}

var known = map[string]service{
	"jellyfin":    {needsUrl: true, needsSecret: true, check: checkJellyfin},
	"seerr":       {needsUrl: true, needsSecret: true, check: checkSeerr},
	"sonarr":      {needsUrl: true, needsSecret: true, check: checkArr},
	"radarr":      {needsUrl: true, needsSecret: true, check: checkArr},
	"qbittorrent": {needsUrl: true, needsSecret: true, check: checkQbittorrent},
	"tdarr":       {needsUrl: true, check: checkTdarr},
	"pihole":      {needsUrl: true, needsSecret: true, check: checkPihole},
}

func Known(name string) bool {
	_, ok := known[name]
	return ok
}

// saves, tries to talk to the service and only then marks it available.
// same as the others, an empty secret keeps the one saved
func Save(name string, c Config) error {
	s, ok := known[name]
	if !ok {
		return fmt.Errorf("unknown service %s", name)
	}

	c.Url = strings.TrimRight(strings.TrimSpace(c.Url), "/")
	c.Username = strings.TrimSpace(c.Username)
	c.Extra = strings.TrimSpace(c.Extra)
	if s.needsUrl && c.Url == "" {
		return fmt.Errorf("url is required")
	}

	if c.Secret == "" {
		old, err := Load(name)
		if err == nil {
			c.Secret = old.Secret
		}
	}
	if s.needsSecret && c.Secret == "" {
		return fmt.Errorf("the key/password is required")
	}

	conn, err := connection.Db()
	if err != nil {
		return err
	}
	query := `
	insert into frontdesk.service_configs (name, url, username, secret, extra)
	values ($1, $2, $3, $4, $5)
	on conflict (name) do update set
		url = excluded.url,
		username = excluded.username,
		secret = excluded.secret,
		extra = excluded.extra`
	_, err = conn.Exec(query, name, c.Url, c.Username, c.Secret, c.Extra)
	if err != nil {
		return err
	}
	forgetSessions(name)

	// saved either way, but it only counts as integrated if it answers
	err = s.check(c)
	if err != nil {
		integrations.SetUnavailable(name)
		return fmt.Errorf("saved, but %s did not answer right: %w", name, err)
	}
	return integrations.SetAvailable(name)
}

func Load(name string) (*Config, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	c := new(Config)
	query := `select url, username, secret, extra from frontdesk.service_configs where name = $1`
	err = conn.QueryRow(query, name).Scan(&c.Url, &c.Username, &c.Secret, &c.Extra)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s is not configured", name)
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func Public(name string) (map[string]interface{}, error) {
	c, err := Load(name)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"url":       c.Url,
		"username":  c.Username,
		"extra":     c.Extra,
		"secretSet": c.Secret != "",
	}, nil
}

// loads the config only if the integration is on, so the widgets get a clear message
func loadEnabled(name string) (*Config, error) {
	enabled, err := integrations.CheckFor(name)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, fmt.Errorf("%s is not configured", name)
	}
	return Load(name)
}

// connection.Api has no timeout, and a dead service would hang the widget forever
var client = &http.Client{Timeout: 15 * time.Second}

func call(c *http.Client, method string, url string, headers map[string]string, body any, out any) error {
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return &statusError{Code: res.StatusCode, Body: string(data)}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

type statusError struct {
	Code int
	Body string
}

func (e *statusError) Error() string {
	body := e.Body
	if len(body) > 200 {
		body = body[:200]
	}
	return fmt.Sprintf("status: %d - %s", e.Code, body)
}

func hasStatus(err error, code int) bool {
	var s *statusError
	return errors.As(err, &s) && s.Code == code
}

// qbittorrent and pihole log in and keep the session instead of logging in on every call
type session struct {
	sync.Mutex
	value   string
	expires time.Time
}

func (s *session) forget() {
	s.Lock()
	s.value = ""
	s.Unlock()
}

// gets the saved session or logs in again when it is gone
func (s *session) get(login func() (string, time.Time, error)) (string, error) {
	s.Lock()
	defer s.Unlock()
	if s.value != "" && time.Until(s.expires) > time.Minute {
		return s.value, nil
	}
	value, expires, err := login()
	if err != nil {
		return "", err
	}
	s.value = value
	s.expires = expires
	return value, nil
}

// drops the session when the config changes
func forgetSessions(name string) {
	switch name {
	case "qbittorrent":
		qbit.forget()
	case "pihole":
		piholeSession.forget()
	}
}
