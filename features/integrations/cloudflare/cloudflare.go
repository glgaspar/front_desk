package cloudflare

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/glgaspar/front_desk/connection"
)

type IngressRule struct {
	Hostname      *string                `json:"hostname,omitempty"`
	Service       string                 `json:"service"`
	OriginRequest map[string]interface{} `json:"originRequest,omitempty"`
}

type tunnelConfigBody struct {
	Config struct {
		Ingress []IngressRule `json:"ingress"`
	} `json:"config"`
}

type cfResponse struct {
	Success    bool            `json:"success"`
	Errors     []interface{}   `json:"errors"`
	Result     json.RawMessage `json:"result"`
	ResultInfo *struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

type DNSRecord struct {
	Id              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Content         string `json:"content"`
	Proxied         bool   `json:"proxied"`
	ManagedByTunnel bool   `json:"managedByTunnel"`
}

type SavedRoute struct {
	Hostname string `json:"hostname"`
	AppDir   string `json:"appDir"`
	Service  string `json:"service"`
}

type Config struct {
	AccountId          string `json:"accountId" db:"accountId"`
	TunnelId           string `json:"tunnelId" db:"tunnelId"`
	CloudflareAPIToken string `json:"cloudflareAPIToken" db:"cloudflareAPIToken"`
	LocalAddress       string `json:"localAddress" db:"localAddress"`
	ZoneId             string `json:"zoneId" db:"zoneId"`
	Enabled            bool   `json:"enabled" db:"enabled"`
}

const api = "https://api.cloudflare.com/client/v4"

func (c *Config) SetCloudflare() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}

	// the front never gets to see the ids or the token, so empty means keep the one we have
	var saved Config
	err = conn.QueryRow(`select accountId, tunnelId, cloudflareAPIToken, zoneId from frontdesk.cloudflare`).Scan(&saved.AccountId, &saved.TunnelId, &saved.CloudflareAPIToken, &saved.ZoneId)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	for _, field := range []struct {
		name         string
		value, saved *string
	}{
		{"account id", &c.AccountId, &saved.AccountId},
		{"tunnel id", &c.TunnelId, &saved.TunnelId},
		{"api token", &c.CloudflareAPIToken, &saved.CloudflareAPIToken},
		{"zone id", &c.ZoneId, &saved.ZoneId},
	} {
		*field.value = strings.TrimSpace(*field.value)
		if *field.value == "" {
			*field.value = *field.saved
		}
		if *field.value == "" {
			return fmt.Errorf("cloudflare %s is required", field.name)
		}
	}
	if strings.TrimSpace(c.LocalAddress) == "" {
		return fmt.Errorf("cloudflare local address is required")
	}

	tran, err := conn.Begin()
	if err != nil {
		return err
	}

	query := `
	delete from frontdesk.cloudflare;
	`
	_, err = tran.Exec(query)
	if err != nil {
		tran.Rollback()
		return err
	}

	query = `
	insert into frontdesk.cloudflare (accountId, tunnelId, cloudflareAPIToken, localAddress, zoneId)
	values($1,$2,$3,$4,$5);
	`
	_, err = tran.Exec(query, c.AccountId, c.TunnelId, c.CloudflareAPIToken, c.LocalAddress, c.ZoneId)
	if err != nil {
		tran.Rollback()
		return err
	}

	return tran.Commit()
}

func loadConfig() (*Config, error) {
	c := new(Config)
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	query := `
	select accountId, tunnelId, cloudflareAPIToken, localAddress, zoneId
	from frontdesk.cloudflare
	`
	err = conn.QueryRow(query).Scan(&c.AccountId, &c.TunnelId, &c.CloudflareAPIToken, &c.LocalAddress, &c.ZoneId)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("cloudflare is not configured")
	}
	if err != nil {
		return nil, err
	}

	for _, val := range []struct {
		name, val string
	}{
		{"cloudflare localAddress", c.LocalAddress},
		{"cloudflare api token", c.CloudflareAPIToken},
		{"cloudflare account id", c.AccountId},
		{"cloudflare tunnel id", c.TunnelId},
		{"cloudflare zone id", c.ZoneId},
	} {
		if val.val == "" {
			return nil, fmt.Errorf("%s is not set", val.name)
		}
	}

	return c, nil
}

// only says what is set, the ids are nobody business
func PublicConfig() (map[string]interface{}, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"localAddress": c.LocalAddress,
		"accountIdSet": c.AccountId != "",
		"tunnelIdSet":  c.TunnelId != "",
		"zoneIdSet":    c.ZoneId != "",
		"tokenSet":     c.CloudflareAPIToken != "",
	}, nil
}

func (c *Config) headers() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + c.CloudflareAPIToken,
		"Content-Type":  "application/json",
	}
}

func (c *Config) call(method string, url string, body any) (*cfResponse, error) {
	res, err := connection.Api(method, url, c.headers(), body)
	if err != nil {
		return nil, err
	}

	var parsed cfResponse
	err = json.Unmarshal(*res, &parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cloudflare response: %w", err)
	}
	if !parsed.Success {
		return nil, fmt.Errorf("cloudflare API error: %+v", parsed.Errors)
	}
	return &parsed, nil
}

func (c *Config) tunnelUrl() string {
	return fmt.Sprintf("%s/accounts/%s/cfd_tunnel/%s/configurations", api, c.AccountId, c.TunnelId)
}

func (c *Config) getIngress() ([]IngressRule, error) {
	res, err := c.call("GET", c.tunnelUrl(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tunnel config: %w", err)
	}

	var body tunnelConfigBody
	err = json.Unmarshal(res.Result, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tunnel config JSON: %w", err)
	}
	return body.Config.Ingress, nil
}

func (c *Config) putIngress(rules []IngressRule) error {
	body := tunnelConfigBody{}
	body.Config.Ingress = rules
	_, err := c.call("PUT", c.tunnelUrl(), body)
	if err != nil {
		return fmt.Errorf("failed to update tunnel ingress config: %w", err)
	}
	return nil
}

func (c *Config) dnsRecords(name string) ([]DNSRecord, error) {
	var records []DNSRecord
	page := 1
	for {
		u := fmt.Sprintf("%s/zones/%s/dns_records?type=CNAME&per_page=100&page=%d", api, c.ZoneId, page)
		if name != "" {
			u += "&name=" + url.QueryEscape(name)
		}
		res, err := c.call("GET", u, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list DNS records: %w", err)
		}

		var pageRecords []DNSRecord
		err = json.Unmarshal(res.Result, &pageRecords)
		if err != nil {
			return nil, err
		}
		records = append(records, pageRecords...)

		if res.ResultInfo == nil || page >= res.ResultInfo.TotalPages {
			break
		}
		page++
	}

	for i := range records {
		records[i].ManagedByTunnel = records[i].Content == c.TunnelId+".cfargotunnel.com"
	}
	return records, nil
}

func ListIngress() ([]IngressRule, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return c.getIngress()
}

func ListDNSRecords() ([]DNSRecord, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return c.dnsRecords("")
}

func ListSavedRoutes() ([]SavedRoute, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(`select hostname, coalesce(app_dir, ''), service from frontdesk.cloudflare_routes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := []SavedRoute{}
	for rows.Next() {
		var r SavedRoute
		err = rows.Scan(&r.Hostname, &r.AppDir, &r.Service)
		if err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, nil
}

// new rules go first, the catch all has to stay at the end or cloudflare yells at you
func insertIngress(rules []IngressRule, rule IngressRule) []IngressRule {
	for _, r := range rules {
		if r.Hostname != nil && rule.Hostname != nil && *r.Hostname == *rule.Hostname {
			return rules
		}
	}
	return append([]IngressRule{rule}, rules...)
}

func filterIngress(rules []IngressRule, hostname string) []IngressRule {
	filtered := []IngressRule{}
	for _, r := range rules {
		if r.Hostname != nil && *r.Hostname == hostname {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func HostFromUrl(appUrl string) string {
	if !strings.Contains(appUrl, "://") {
		appUrl = "https://" + appUrl
	}
	u, err := url.Parse(appUrl)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func AddHostname(hostname string, localPort string, appDir string) error {
	if hostname == "" || localPort == "" {
		return fmt.Errorf("hostname and port are required")
	}

	c, err := loadConfig()
	if err != nil {
		return err
	}

	ingress, err := c.getIngress()
	if err != nil {
		return err
	}

	service := "http://" + c.LocalAddress + ":" + localPort
	err = c.putIngress(insertIngress(ingress, IngressRule{Hostname: &hostname, Service: service}))
	if err != nil {
		return err
	}

	// looking before creating, posting the same record twice is an error
	records, err := c.dnsRecords(hostname)
	if err != nil {
		return err
	}

	recordId := ""
	if len(records) > 0 {
		recordId = records[0].Id
	} else {
		payload := map[string]interface{}{
			"type":    "CNAME",
			"name":    hostname,
			"content": c.TunnelId + ".cfargotunnel.com",
			"ttl":     1,
			"proxied": true,
		}
		res, err := c.call("POST", fmt.Sprintf("%s/zones/%s/dns_records", api, c.ZoneId), payload)
		if err != nil {
			return fmt.Errorf("failed to create DNS record: %w", err)
		}
		var record DNSRecord
		json.Unmarshal(res.Result, &record)
		recordId = record.Id
	}

	conn, err := connection.Db()
	if err != nil {
		return err
	}
	query := `
	insert into frontdesk.cloudflare_routes (hostname, app_dir, service, dns_record_id)
	values ($1, nullif($2, ''), $3, nullif($4, ''))
	on conflict (hostname) do update set
		app_dir = excluded.app_dir,
		service = excluded.service,
		dns_record_id = excluded.dns_record_id`
	_, err = conn.Exec(query, hostname, appDir, service, recordId)
	return err
}

func RemoveHostname(hostname string, removeDNS bool) error {
	if hostname == "" {
		return fmt.Errorf("hostname is required")
	}

	c, err := loadConfig()
	if err != nil {
		return err
	}

	ingress, err := c.getIngress()
	if err != nil {
		return err
	}

	filtered := filterIngress(ingress, hostname)
	if len(filtered) != len(ingress) {
		err = c.putIngress(filtered)
		if err != nil {
			return err
		}
	}

	if removeDNS {
		records, err := c.dnsRecords(hostname)
		if err != nil {
			return err
		}
		for _, record := range records {
			// only the ones pointing to our tunnel, not touching anything else
			if !record.ManagedByTunnel {
				continue
			}
			_, err = c.call("DELETE", fmt.Sprintf("%s/zones/%s/dns_records/%s", api, c.ZoneId, record.Id), nil)
			if err != nil {
				return fmt.Errorf("failed to delete DNS record: %w", err)
			}
		}
	}

	conn, err := connection.Db()
	if err != nil {
		return err
	}
	_, err = conn.Exec(`delete from frontdesk.cloudflare_routes where hostname = $1`, hostname)
	return err
}

// cleans whatever we created for that app. if it was created before we started saving routes
// it tries the app url, but only if the service matches the port
func RemoveForApp(appDir string, hostname string, port string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}

	rows, err := conn.Query(`select hostname from frontdesk.cloudflare_routes where app_dir = $1`, appDir)
	if err != nil {
		return err
	}
	var hostnames []string
	for rows.Next() {
		var h string
		rows.Scan(&h)
		hostnames = append(hostnames, h)
	}
	rows.Close()

	if len(hostnames) == 0 && hostname != "" && port != "" {
		ingress, err := ListIngress()
		if err != nil {
			return err
		}
		for _, r := range ingress {
			if r.Hostname != nil && *r.Hostname == hostname && strings.HasSuffix(r.Service, ":"+port) {
				hostnames = append(hostnames, hostname)
			}
		}
	}

	for _, h := range hostnames {
		err = RemoveHostname(h, true)
		if err != nil {
			return err
		}
	}
	return nil
}
