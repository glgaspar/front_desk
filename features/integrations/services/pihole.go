package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

var piholeSession session

type History struct {
	Timestamp int64 `json:"timestamp"`
	Total     int   `json:"total"`
	Cached    int   `json:"cached"`
	Blocked   int   `json:"blocked"`
	Forwarded int   `json:"forwarded"`
}

type HistoryWrapper struct {
	History []History `json:"history"`
	Took    float64   `json:"took"`
}

// people paste the admin page url, the api lives next to it
func piholeApi(c *Config) string {
	return strings.TrimSuffix(c.Url, "/admin") + "/api"
}

func piholeLogin(c *Config) (string, time.Time, error) {
	var res struct {
		Session struct {
			Valid    bool   `json:"valid"`
			Sid      string `json:"sid"`
			Validity int    `json:"validity"`
			Message  string `json:"message"`
		} `json:"session"`
	}
	err := call(client, "POST", piholeApi(c)+"/auth", nil, map[string]string{"password": c.Secret}, &res)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("pihole login failed: %w", err)
	}
	if !res.Session.Valid {
		return "", time.Time{}, fmt.Errorf("pihole login failed: %s", res.Session.Message)
	}
	// pihole only allows a few sessions at once, so this one gets reused until it expires
	return res.Session.Sid, time.Now().Add(time.Duration(res.Session.Validity) * time.Second), nil
}

func checkPihole(c Config) error {
	_, _, err := piholeLogin(&c)
	return err
}

func pihole(c *Config, method string, path string, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		sid, err := piholeSession.get(func() (string, time.Time, error) { return piholeLogin(c) })
		if err != nil {
			return err
		}
		err = call(client, method, piholeApi(c)+path, map[string]string{"X-FTL-SID": sid}, nil, out)
		if hasStatus(err, http.StatusUnauthorized) {
			piholeSession.forget()
			continue
		}
		if err != nil {
			return fmt.Errorf("pihole: %w", err)
		}
		return nil
	}
	return fmt.Errorf("pihole keeps saying unauthorized, check the password")
}

func PiholeHistory() (*HistoryWrapper, error) {
	c, err := loadEnabled("pihole")
	if err != nil {
		return nil, err
	}

	var res struct {
		History []struct {
			Timestamp float64 `json:"timestamp"`
			Total     int     `json:"total"`
			Cached    int     `json:"cached"`
			Blocked   int     `json:"blocked"`
			Forwarded int     `json:"forwarded"`
		} `json:"history"`
		Took float64 `json:"took"`
	}
	err = pihole(c, "GET", "/history", &res)
	if err != nil {
		return nil, err
	}

	// pihole sends seconds, the front wants milliseconds like everything else in js
	wrapper := &HistoryWrapper{History: []History{}, Took: res.Took}
	for _, h := range res.History {
		wrapper.History = append(wrapper.History, History{
			Timestamp: int64(h.Timestamp * 1000),
			Total:     h.Total,
			Cached:    h.Cached,
			Blocked:   h.Blocked,
			Forwarded: h.Forwarded,
		})
	}
	return wrapper, nil
}
