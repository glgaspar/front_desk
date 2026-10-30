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
