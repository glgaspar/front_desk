package services

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Torrent struct {
	Hash     string  `json:"hash"`
	Name     string  `json:"name"`
	Progress float64 `json:"progress"`
	DlSpeed  int64   `json:"dlspeed"`
	UpSpeed  int64   `json:"upspeed"`
	State    string  `json:"state"`
}

var qbit session

// qbittorrent wants a form, not json, and answers with a cookie
func qbitPost(c *Config, path string, form url.Values, sid string) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.Url+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if sid != "" {
		req.AddCookie(&http.Cookie{Name: "SID", Value: sid})
	}
	return client.Do(req)
}

func qbitLogin(c *Config) (string, time.Time, error) {
	res, err := qbitPost(c, "/api/v2/auth/login", url.Values{"username": {c.Username}, "password": {c.Secret}}, "")
	if err != nil {
		return "", time.Time{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	for _, cookie := range res.Cookies() {
		if cookie.Name == "SID" {
			// the webui default session timeout is one hour
			return cookie.Value, time.Now().Add(50 * time.Minute), nil
		}
	}
	return "", time.Time{}, fmt.Errorf("qbittorrent login failed: %d %s", res.StatusCode, strings.TrimSpace(string(body)))
}

func checkQbittorrent(c Config) error {
	_, _, err := qbitLogin(&c)
	return err
}

// does the call and logs in again once if the session died on the other side
func qbitDo(c *Config, method string, path string, form url.Values, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		sid, err := qbit.get(func() (string, time.Time, error) { return qbitLogin(c) })
		if err != nil {
			return err
		}

		if method == "GET" {
			err = call(client, "GET", c.Url+path, map[string]string{"Cookie": "SID=" + sid}, nil, out)
		} else {
			var res *http.Response
			res, err = qbitPost(c, path, form, sid)
			if err == nil {
				body, _ := io.ReadAll(res.Body)
				res.Body.Close()
				if res.StatusCode != http.StatusOK {
					err = &statusError{Code: res.StatusCode, Body: string(body)}
				}
			}
		}

		if hasStatus(err, http.StatusForbidden) {
			qbit.forget()
			continue
		}
		return err
	}
	return fmt.Errorf("qbittorrent keeps saying forbidden, check the user and password")
}

func QbitTorrents() ([]Torrent, error) {
	c, err := loadEnabled("qbittorrent")
	if err != nil {
		return nil, err
	}
	torrents := []Torrent{}
	err = qbitDo(c, "GET", "/api/v2/torrents/info", nil, &torrents)
	return torrents, err
}

func QbitAction(hash string, action string) error {
	c, err := loadEnabled("qbittorrent")
	if err != nil {
		return err
	}

	form := url.Values{"hashes": {hash}}
	switch action {
	case "remove":
		form.Set("deleteFiles", "false")
		return qbitDo(c, "POST", "/api/v2/torrents/delete", form, nil)
	case "start", "stop":
		err = qbitDo(c, "POST", "/api/v2/torrents/"+action, form, nil)
		// before 5.0 it was resume and pause
		if hasStatus(err, http.StatusNotFound) {
			old := map[string]string{"start": "resume", "stop": "pause"}[action]
			err = qbitDo(c, "POST", "/api/v2/torrents/"+old, form, nil)
		}
		return err
	}
	return fmt.Errorf("unknown action: %s", action)
}
