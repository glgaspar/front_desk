package npm

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/glgaspar/front_desk/features/integrations/services"
)

type Config struct {
	Url                  string `json:"url"`
	Email                string `json:"email"`
	Password             string `json:"password"`
	ForwardHost          string `json:"forwardHost"`
	DefaultCertificateId int    `json:"defaultCertificateId"`
}

// npm sends booleans as true/false or 1/0 depending on the version, so we take both
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	*b = flexBool(s == "true" || s == "1")
	return nil
}

type ProxyHost struct {
	Id            int      `json:"id"`
	DomainNames   []string `json:"domain_names"`
	ForwardScheme string   `json:"forward_scheme"`
	ForwardHost   string   `json:"forward_host"`
	ForwardPort   int      `json:"forward_port"`
	CertificateId int      `json:"certificate_id"`
	SslForced     flexBool `json:"ssl_forced"`
	Enabled       flexBool `json:"enabled"`
}

type Certificate struct {
	Id          int      `json:"id"`
	NiceName    string   `json:"nice_name"`
	Provider    string   `json:"provider"`
	DomainNames []string `json:"domain_names"`
	ExpiresOn   string   `json:"expires_on"`
}

type SavedRoute struct {
	ProxyHostId int    `json:"proxyHostId"`
	Hostname    string `json:"hostname"`
	AppDir      string `json:"appDir"`
}

func (c *Config) SetNPM() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}

	c.Url = strings.TrimRight(strings.TrimSpace(c.Url), "/")
	if c.Url == "" || c.Email == "" || c.ForwardHost == "" {
		return fmt.Errorf("url, email and forward host are required")
	}

	// same as cloudflare, empty password keeps the one saved
	if c.Password == "" {
		err = conn.QueryRow(`select password from frontdesk.nginx_proxy_manager`).Scan(&c.Password)
		if err == sql.ErrNoRows {
			return fmt.Errorf("password is required")
		}
		if err != nil {
			return err
		}
	}

	tran, err := conn.Begin()
	if err != nil {
		return err
	}

	_, err = tran.Exec(`delete from frontdesk.nginx_proxy_manager;`)
	if err != nil {
		tran.Rollback()
		return err
	}

	query := `
	insert into frontdesk.nginx_proxy_manager (url, email, password, forward_host, default_certificate_id)
	values ($1, $2, $3, $4, $5);
	`
	_, err = tran.Exec(query, c.Url, c.Email, c.Password, c.ForwardHost, c.DefaultCertificateId)
	if err != nil {
		tran.Rollback()
		return err
	}

	err = tran.Commit()
	if err != nil {
		return err
	}

	forgetToken()
	return nil
}

func loadConfig() (*Config, error) {
	c := new(Config)
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	query := `
	select url, email, password, forward_host, default_certificate_id
	from frontdesk.nginx_proxy_manager
	`
	err = conn.QueryRow(query).Scan(&c.Url, &c.Email, &c.Password, &c.ForwardHost, &c.DefaultCertificateId)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("nginx proxy manager is not configured")
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func PublicConfig() (map[string]interface{}, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"url":                  c.Url,
		"email":                c.Email,
		"forwardHost":          c.ForwardHost,
		"defaultCertificateId": c.DefaultCertificateId,
		"passwordSet":          c.Password != "",
	}, nil
}

func DefaultCertificate() int {
	c, err := loadConfig()
	if err != nil {
		return 0
	}
	return c.DefaultCertificateId
}

var tokenCache struct {
	sync.Mutex
	url     string
	token   string
	expires time.Time
}

func forgetToken() {
	tokenCache.Lock()
	tokenCache.token = ""
	tokenCache.Unlock()
}

// npm tokens last a while, no reason to log in on every request
func (c *Config) token() (string, error) {
	tokenCache.Lock()
	defer tokenCache.Unlock()

	if tokenCache.token != "" && tokenCache.url == c.Url && time.Until(tokenCache.expires) > 5*time.Minute {
		return tokenCache.token, nil
	}

	res, err := connection.Api("POST", c.Url+"/api/tokens", map[string]string{"Content-Type": "application/json"}, map[string]string{
		"identity": c.Email,
		"secret":   c.Password,
	})
	if err != nil {
		return "", fmt.Errorf("could not log in on nginx proxy manager: %w", err)
	}

	var parsed struct {
		Token   string    `json:"token"`
		Expires time.Time `json:"expires"`
	}
	err = json.Unmarshal(*res, &parsed)
	if err != nil || parsed.Token == "" {
		return "", fmt.Errorf("nginx proxy manager did not send a token back")
	}
	if parsed.Expires.IsZero() {
		parsed.Expires = time.Now().Add(time.Hour)
	}

	tokenCache.url = c.Url
	tokenCache.token = parsed.Token
	tokenCache.expires = parsed.Expires
	return parsed.Token, nil
}

func (c *Config) call(method string, path string, body any, out any) error {
	token, err := c.token()
	if err != nil {
		return err
	}

	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}
	res, err := connection.Api(method, c.Url+path, headers, body)
	if err != nil {
		return fmt.Errorf("nginx proxy manager: %w", err)
	}
	if out != nil {
		err = json.Unmarshal(*res, out)
		if err != nil {
			return fmt.Errorf("could not read nginx proxy manager response: %w", err)
		}
	}
	return nil
}

func (c *Config) proxyHosts() ([]ProxyHost, error) {
	var hosts []ProxyHost
	err := c.call("GET", "/api/nginx/proxy-hosts", nil, &hosts)
	return hosts, err
}

func ListProxyHosts() ([]ProxyHost, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return c.proxyHosts()
}

func ListCertificates() ([]Certificate, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	certs := []Certificate{}
	err = c.call("GET", "/api/nginx/certificates", nil, &certs)
	return certs, err
}

// npm happily accepts a certificate that doesnt exist, so we check it ourselves
func (c *Config) checkCertificate(certId int) error {
	if certId <= 0 {
		return nil
	}
	var certs []Certificate
	err := c.call("GET", "/api/nginx/certificates", nil, &certs)
	if err != nil {
		return err
	}
	for _, cert := range certs {
		if cert.Id == certId {
			return nil
		}
	}
	return fmt.Errorf("certificate %d does not exist on nginx proxy manager", certId)
}

func ListSavedRoutes() ([]SavedRoute, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(`select proxy_host_id, hostname, coalesce(app_dir, '') from frontdesk.npm_routes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := []SavedRoute{}
	for rows.Next() {
		var r SavedRoute
		err = rows.Scan(&r.ProxyHostId, &r.Hostname, &r.AppDir)
		if err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, nil
}

func findHost(hosts []ProxyHost, hostname string) *ProxyHost {
	for i, h := range hosts {
		for _, domain := range h.DomainNames {
			if strings.EqualFold(domain, hostname) {
				return &hosts[i]
			}
		}
	}
	return nil
}

func withCertificate(certId int) map[string]interface{} {
	return map[string]interface{}{
		"certificate_id": certId,
		"ssl_forced":     certId > 0,
		"http2_support":  certId > 0,
	}
}

func hostPayload(hostname string, forwardHost string, port int, certId int) map[string]interface{} {
	payload := map[string]interface{}{
		"domain_names":            []string{hostname},
		"forward_scheme":          "http",
		"forward_host":            forwardHost,
		"forward_port":            port,
		"block_exploits":          true,
		"allow_websocket_upgrade": true,
		"access_list_id":          0,
		"caching_enabled":         false,
		"advanced_config":         "",
		"meta":                    map[string]interface{}{},
		"locations":               []interface{}{},
	}
	for k, v := range withCertificate(certId) {
		payload[k] = v
	}
	return payload
}

func AddProxyHost(hostname string, localPort string, appDir string, certId int) (*ProxyHost, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	port, err := strconv.Atoi(localPort)
	if hostname == "" || err != nil {
		return nil, fmt.Errorf("hostname and a valid port are required")
	}

	c, err := loadConfig()
	if err != nil {
		return nil, err
	}

	err = c.checkCertificate(certId)
	if err != nil {
		return nil, err
	}

	hosts, err := c.proxyHosts()
	if err != nil {
		return nil, err
	}

	var host ProxyHost
	existing := findHost(hosts, hostname)
	if existing != nil {
		// already there, just point it to the right place
		update := withCertificate(certId)
		update["forward_host"] = c.ForwardHost
		update["forward_port"] = port
		err = c.call("PUT", fmt.Sprintf("/api/nginx/proxy-hosts/%d", existing.Id), update, &host)
	} else {
		err = c.call("POST", "/api/nginx/proxy-hosts", hostPayload(hostname, c.ForwardHost, port, certId), &host)
	}
	if err != nil {
		return nil, err
	}

	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}
	query := `
	insert into frontdesk.npm_routes (proxy_host_id, hostname, app_dir)
	values ($1, $2, nullif($3, ''))
	on conflict (proxy_host_id) do update set
		hostname = excluded.hostname,
		app_dir = coalesce(excluded.app_dir, frontdesk.npm_routes.app_dir)`
	_, err = conn.Exec(query, host.Id, hostname, appDir)
	if err != nil {
		return nil, err
	}

	// so the lan goes straight to npm instead of out through cloudflare. the host is
	// already made, a pihole problem shouldnt fail the whole thing
	if services.PiholeEnabled() {
		if err := services.AddDns(hostname); err != nil {
			log.Printf("could not add %s to pihole: %v", hostname, err)
		}
	}

	return &host, nil
}

func SetCertificate(hostId int, certId int) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	err = c.checkCertificate(certId)
	if err != nil {
		return err
	}
	return c.call("PUT", fmt.Sprintf("/api/nginx/proxy-hosts/%d", hostId), withCertificate(certId), nil)
}

func RemoveProxyHost(hostId int) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}

	// need the domains before its gone to clean them from pihole too
	var host ProxyHost
	c.call("GET", fmt.Sprintf("/api/nginx/proxy-hosts/%d", hostId), nil, &host)

	err = c.call("DELETE", fmt.Sprintf("/api/nginx/proxy-hosts/%d", hostId), nil, nil)
	// someone already deleted it on npm, just forget about it here too
	if err != nil && !strings.Contains(err.Error(), "status: 404") {
		return err
	}

	conn, err := connection.Db()
	if err != nil {
		return err
	}
	_, err = conn.Exec(`delete from frontdesk.npm_routes where proxy_host_id = $1`, hostId)
	if err != nil {
		return err
	}

	if services.PiholeEnabled() {
		for _, domain := range host.DomainNames {
			if err := services.RemoveDns(domain, true); err != nil {
				log.Printf("could not remove %s from pihole: %v", domain, err)
			}
		}
	}
	return nil
}

// same as cloudflare, removes what we created for that app. for older apps it only
// removes a host that matches the url AND forwards to the app port
func RemoveForApp(appDir string, hostname string, port string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}

	rows, err := conn.Query(`select proxy_host_id from frontdesk.npm_routes where app_dir = $1`, appDir)
	if err != nil {
		return err
	}
	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	if len(ids) == 0 && hostname != "" && port != "" {
		hosts, err := ListProxyHosts()
		if err != nil {
			return err
		}
		host := findHost(hosts, hostname)
		if host != nil && strconv.Itoa(host.ForwardPort) == port {
			ids = append(ids, host.Id)
		}
	}

	for _, id := range ids {
		err = RemoveProxyHost(id)
		if err != nil {
			return err
		}
	}
	return nil
}
