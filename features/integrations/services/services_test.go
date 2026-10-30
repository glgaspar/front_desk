package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestPiholeSessionAndDns(t *testing.T) {
	piholeSession.forget()
	logins := 0
	var deleted, added []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/auth":
			logins++
			json.NewEncoder(w).Encode(map[string]interface{}{"session": map[string]interface{}{"valid": true, "sid": "sid1", "validity": 1800}})
		case r.Header.Get("X-FTL-SID") != "sid1":
			w.WriteHeader(401)
		case r.Method == "GET" && r.URL.Path == "/api/config/dns/hosts":
			w.Write([]byte(`{"config":{"dns":{"hosts":["10.0.0.5 old.example.com","192.168.0.10 app.example.com alias.example.com"]}}}`))
		case r.Method == "DELETE":
			p, _ := url.PathUnescape(r.URL.EscapedPath())
			deleted = append(deleted, p)
			w.WriteHeader(204)
		case r.Method == "PUT":
			p, _ := url.PathUnescape(r.URL.EscapedPath())
			added = append(added, p)
			w.WriteHeader(201)
		}
	}))
	defer server.Close()

	// the admin suffix is what people paste, it has to go
	c := &Config{Url: server.URL + "/admin", Secret: "pw", Extra: "192.168.0.10"}
	records, err := dnsRecords(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || records[2].Hostname != "alias.example.com" || records[2].Ip != "192.168.0.10" {
		t.Fatalf("got %+v", records)
	}

	// same session reused
	dnsRecords(c)
	if logins != 1 {
		t.Errorf("logged in %d times", logins)
	}

	// deleting an alias deletes the whole line, the way pihole stores it
	err = pihole(c, "DELETE", dnsEntry(records[2]), nil)
	if err != nil || deleted[0] != "/api/config/dns/hosts/192.168.0.10 app.example.com alias.example.com" {
		t.Errorf("got %v %v", err, deleted)
	}

	err = pihole(c, "PUT", dnsEntry(DnsRecord{Ip: "192.168.0.10", Hostname: "new.example.com"}), nil)
	if err != nil || added[0] != "/api/config/dns/hosts/192.168.0.10 new.example.com" {
		t.Errorf("got %v %v", err, added)
	}

	// session died on pihole, logs in again by itself
	piholeSession.value = "dead"
	_, err = dnsRecords(c)
	if err != nil || logins != 2 {
		t.Errorf("got %v after %d logins", err, logins)
	}
}

func TestQbitRelogin(t *testing.T) {
	qbit.forget()
	logins := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			logins++
			r.ParseForm()
			if r.Form.Get("password") != "pw" {
				w.Write([]byte("Fails."))
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "good"})
			w.Write([]byte("Ok."))
			return
		}
		if cookie, err := r.Cookie("SID"); err != nil || cookie.Value != "good" {
			w.WriteHeader(403)
			return
		}
		switch r.URL.Path {
		case "/api/v2/torrents/info":
			w.Write([]byte(`[{"hash":"abc","name":"debian.iso","progress":0.5,"dlspeed":100,"upspeed":5,"state":"downloading"}]`))
		case "/api/v2/torrents/stop":
			// older than 5.0
			w.WriteHeader(404)
		case "/api/v2/torrents/pause":
			w.Write([]byte(""))
		}
	}))
	defer server.Close()

	c := &Config{Url: server.URL, Username: "admin", Secret: "pw"}
	qbit.value = "expired"
	qbit.expires = qbit.expires.AddDate(1000, 0, 0)

	var torrents []Torrent
	err := qbitDo(c, "GET", "/api/v2/torrents/info", nil, &torrents)
	if err != nil || len(torrents) != 1 || torrents[0].Name != "debian.iso" || logins != 1 {
		t.Fatalf("got %v %+v after %d logins", err, torrents, logins)
	}

	err = qbitDo(c, "POST", "/api/v2/torrents/stop", url.Values{"hashes": {"abc"}}, nil)
	if !hasStatus(err, 404) {
		t.Errorf("stop should 404 on old versions, got %v", err)
	}

	if err := checkQbittorrent(Config{Url: server.URL, Secret: "wrong"}); err == nil {
		t.Error("wrong password should fail")
	}
}

func TestArrQueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "key" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"records":[
			{"title":"Show.S01E02.1080p","size":1000,"sizeleft":250,"timeleft":"00:10:00","status":"downloading","series":{"title":"Show"},"episode":{"seasonNumber":1,"episodeNumber":2}},
			{"title":"Movie.2024","size":0,"sizeleft":0,"status":"queued","movie":{"title":"Movie","year":2024}}
		]}`))
	}))
	defer server.Close()

	items, err := queue(&Config{Url: server.URL, Secret: "key"}, "sonarr", "/api/v3/queue")
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Title != "Show S01E02" || items[0].Progress != 0.75 || items[0].TimeLeft != "00:10:00" {
		t.Errorf("got %+v", items[0])
	}
	if items[1].Title != "Movie (2024)" || items[1].Progress != 0 {
		t.Errorf("got %+v", items[1])
	}

	_, err = queue(&Config{Url: server.URL, Secret: "nope"}, "radarr", "/api/v3/queue")
	if err == nil || !hasStatus(err, 401) {
		t.Errorf("got %v", err)
	}
}

func TestTdarrStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Data map[string]string `json:"data"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/api/v2/cruddb" || body.Data["docID"] != "statistics" || r.Header.Get("x-api-key") != "k" {
			w.WriteHeader(400)
			return
		}
		w.Write([]byte(`{"totalFileCount":100,"sizeDiff":12.5,"table1Count":3,"table2Count":40,"table3Count":1,"table4Count":2,"table5Count":50,"table6Count":0}`))
	}))
	defer server.Close()

	stats, err := tdarrStats(&Config{Url: server.URL, Secret: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Queue != 5 || stats.Processed != 90 || stats.Errored != 1 || stats.SavedGB != 12.5 || stats.Files != 100 {
		t.Errorf("got %+v", stats)
	}
}
