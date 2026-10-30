package services

import (
	"fmt"
	"net/http"
	"net/url"
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

type DnsRecord struct {
	Ip       string `json:"ip"`
	Hostname string `json:"hostname"`
	line     string
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

func PiholeLanIp() string {
	c, err := Load("pihole")
	if err != nil {
		return ""
	}
	return c.Extra
}

func PiholeEnabled() bool {
	_, err := loadEnabled("pihole")
	return err == nil
}

func dnsRecords(c *Config) ([]DnsRecord, error) {
	var res struct {
		Config struct {
			Dns struct {
				Hosts []string `json:"hosts"`
			} `json:"dns"`
		} `json:"config"`
	}
	err := pihole(c, "GET", "/config/dns/hosts", &res)
	if err != nil {
		return nil, err
	}

	records := []DnsRecord{}
	for _, line := range res.Config.Dns.Hosts {
		// "ip host alias alias..."
		fields := strings.Fields(line)
		for _, host := range fields[min(1, len(fields)):] {
			records = append(records, DnsRecord{Ip: fields[0], Hostname: host, line: line})
		}
	}
	return records, nil
}

func ListDns() ([]DnsRecord, error) {
	c, err := loadEnabled("pihole")
	if err != nil {
		return nil, err
	}
	return dnsRecords(c)
}

// deletes need the whole line as pihole has it, aliases included
func dnsEntry(r DnsRecord) string {
	line := r.line
	if line == "" {
		line = r.Ip + " " + r.Hostname
	}
	return "/config/dns/hosts/" + url.PathEscape(line)
}

// points the hostname to the lan ip from the config. an old record for the same
// name pointing somewhere else gets replaced
func AddDns(hostname string) error {
	c, err := loadEnabled("pihole")
	if err != nil {
		return err
	}
	if c.Extra == "" {
		return fmt.Errorf("set the LAN ip on the pihole config first")
	}
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname == "" {
		return fmt.Errorf("hostname is required")
	}

	records, err := dnsRecords(c)
	if err != nil {
		return err
	}
	for _, r := range records {
		if !strings.EqualFold(r.Hostname, hostname) {
			continue
		}
		if r.Ip == c.Extra {
			return nil
		}
		err = pihole(c, "DELETE", dnsEntry(r), nil)
		if err != nil {
			return err
		}
	}

	return pihole(c, "PUT", dnsEntry(DnsRecord{Ip: c.Extra, Hostname: hostname}), nil)
}

// onlyOurs leaves alone records that point somewhere else than the lan ip, those
// were not made by front desk
func RemoveDns(hostname string, onlyOurs bool) error {
	c, err := loadEnabled("pihole")
	if err != nil {
		return err
	}

	records, err := dnsRecords(c)
	if err != nil {
		return err
	}
	for _, r := range records {
		if !strings.EqualFold(r.Hostname, hostname) || (onlyOurs && r.Ip != c.Extra) {
			continue
		}
		err = pihole(c, "DELETE", dnsEntry(r), nil)
		if err != nil {
			return err
		}
	}
	return nil
}

type PiholeTop struct {
	Total          int     `json:"total"`
	Blocked        int     `json:"blocked"`
	PercentBlocked float64 `json:"percentBlocked"`
	Domains        []struct {
		Domain string `json:"domain"`
		Count  int    `json:"count"`
	} `json:"domains"`
	Clients []struct {
		Ip    string `json:"ip"`
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"clients"`
}

// the day so far: how much got blocked, what got blocked the most and who asks the most
func GetPiholeTop() (*PiholeTop, error) {
	c, err := loadEnabled("pihole")
	if err != nil {
		return nil, err
	}

	top := &PiholeTop{}
	var summary struct {
		Queries struct {
			Total          int     `json:"total"`
			Blocked        int     `json:"blocked"`
			PercentBlocked float64 `json:"percent_blocked"`
		} `json:"queries"`
	}
	err = pihole(c, "GET", "/stats/summary", &summary)
	if err != nil {
		return nil, err
	}
	top.Total = summary.Queries.Total
	top.Blocked = summary.Queries.Blocked
	top.PercentBlocked = summary.Queries.PercentBlocked

	err = pihole(c, "GET", "/stats/top_domains?blocked=true&count=8", top)
	if err != nil {
		return nil, err
	}
	err = pihole(c, "GET", "/stats/top_clients?count=8", top)
	if err != nil {
		return nil, err
	}
	return top, nil
}
