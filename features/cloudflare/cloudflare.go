package cloudflare

import (
	"fmt"
	"os"

	"github.com/glgaspar/front_desk/connection"
)

type ingressRule struct {
	Hostname *string `json:"hostname,omitempty"`
	Service  string  `json:"service"`
	OriginRequest map[string]interface{} `json:"originRequest,omitempty"`
}

type tunnelConfigBody struct {
	Config struct {
		Ingress []ingressRule `json:"ingress"`
	} `json:"config"`
}


type Config struct {
	AccountId          string `json:"accountId" db:"accountId"`
	TunnelId           string `json:"tunnelId" db:"tunnelId"`
	CloudflareAPIToken string `json:"cloudflareAPIToken" db:"cloudflareAPIToken"`
	LocalAddress       string `json:"localAddress" db:"localAddress"`
	ZoneId             string `json:"zoneId" db:"zoneId"`
}

func (c *Config) SetCloudflare() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	delete from adm.cloudflare
	insert into adm.cloudflare (accountId, tunnelId, cloudflareAPIToken, localAddress, zoneId)
	values($1,$2,$3,$4,$5)
	`

	_, err = conn.Exec(query, c.AccountId, c.TunnelId, c.CloudflareAPIToken, c.LocalAddress, c.ZoneId)
	if err != nil {
		return err
	}

	os.Setenv("CLOUDFLARE", "TRUE")
	os.Setenv("CLOUDFLARE_ACCOUNT_ID", c.AccountId)
	os.Setenv("CLOUDFLARE_TUNNEL_ID", c.TunnelId)
	os.Setenv("CLOUDFLARE_API_TOKEN", c.CloudflareAPIToken)
	os.Setenv("CLOUDFLARE_LOCAL_ADDRESS", c.LocalAddress)
	os.Setenv("CLOUDFLARE_ZONE_ID", c.ZoneId)


	return nil
}

func (c *Config) CheckForCloudflare() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	select accountId, tunnelId, cloudflareAPIToken, localAddress, zoneId
	from adm.cloudflare
	`

	rows, err := conn.Query(query)
	if err != nil {
		return err
	}

	for rows.Next() {
		rows.Scan(&c.AccountId, &c.TunnelId, &c.CloudflareAPIToken, &c.LocalAddress, &c.ZoneId)
	}

	if c.AccountId != "" && c.TunnelId != "" && c.CloudflareAPIToken != "" && c.LocalAddress != "" {
		os.Setenv("CLOUDFLARE", "TRUE")
		os.Setenv("CLOUDFLARE_ACCOUNT_ID", c.AccountId)
		os.Setenv("CLOUDFLARE_TUNNEL_ID", c.TunnelId)
		os.Setenv("CLOUDFLARE_API_TOKEN", c.CloudflareAPIToken)
		os.Setenv("CLOUDFLARE_LOCAL_ADDRESS", c.LocalAddress)
		os.Setenv("CLOUDFLARE_ZONE_ID", c.ZoneId)
	}
	return nil
}

func (c *Config) CreateTunnel(hostname string, localPort string) error {
	if os.Getenv("CLOUDFLARE") != "TRUE" {
		return fmt.Errorf("cloudflare is not set up")
	}

	localAddress := os.Getenv("CLOUDFLARE_LOCAL_ADDRESS")
	if localAddress == "" {
		return fmt.Errorf("cloudflare localAddress is not set")
	}

	token := os.Getenv("CLOUDFLARE_API_TOKEN")
	if token == "" {
		return fmt.Errorf("cloudflare api token is not set")
	}

	accountId := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	if accountId == "" {
		return fmt.Errorf("cloudflare account id is not set")
	}

	tunnelId := os.Getenv("CLOUDFLARE_TUNNEL_ID")
	if tunnelId == "" {
		return fmt.Errorf("cloudflare tunnel id is not set")
	}

	zoneId := os.Getenv("CLOUDFLARE_ZONE_ID")
	if zoneId == "" {
		return fmt.Errorf("cloudflare zone id is not set")
	}


	ingress := []ingressRule{
		{
			Hostname: &hostname,
			Service:  "http://" + localAddress + ":" + localPort,
			OriginRequest: map[string]interface{}{"noTLSVerify": true},
		},
		{
			Service: "http_status:404",
		},
	}

	body := tunnelConfigBody{}
	body.Config.Ingress = ingress
	url := fmt.Sprintf(
        "https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations",
        accountId, tunnelId,
    )
	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}
	
	_, err := connection.Api("PUT", url, headers, body)
	if err != nil {
		return err
	}

	url = fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneId)
	payload := map[string]interface{}{
		"type":    "CNAME",
		"name":    hostname,
		"content": tunnelId + ".cfargotunnel.com",
		"ttl":     1,   // Auto
		"proxied": true,
	}
	_, err = connection.Api("POST", url, headers, payload)
	if err != nil {
		return err
	}
	
	return nil
}
