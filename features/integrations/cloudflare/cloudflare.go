package cloudflare

import (
	"encoding/json"
	"fmt"

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

type tunnelConfigResponse struct {
	Success bool               `json:"success"`
	Errors  []interface{}      `json:"errors"`
	Result  tunnelConfigBody `json:"result"`
}

type Config struct {
	AccountId          string `json:"accountId" db:"accountId"`
	TunnelId           string `json:"tunnelId" db:"tunnelId"`
	CloudflareAPIToken string `json:"cloudflareAPIToken" db:"cloudflareAPIToken"`
	LocalAddress       string `json:"localAddress" db:"localAddress"`
	ZoneId             string `json:"zoneId" db:"zoneId"`
	Enabled            bool   `json:"enabled" db:"enabled"`
}

func (c *Config) SetCloudflare() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	tran, err := conn.Begin()
	if err != nil {
		return err
	}

	query := `
	delete from adm.cloudflare;
	`
	_, err = tran.Exec(query)
	if err != nil {
		tran.Rollback()
		return err 
	}

	query = `
	insert into adm.cloudflare (accountId, tunnelId, cloudflareAPIToken, localAddress, zoneId)
	values($1,$2,$3,$4,$5);
	`
	_, err = tran.Exec(query, c.AccountId, c.TunnelId, c.CloudflareAPIToken, c.LocalAddress, c.ZoneId)
	if err != nil {
		tran.Rollback()
		return err 
	}
	
	return tran.Commit()
}

func (c *Config) CheckForCloudflare() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	select enabled
	from adm.integrations_available
	where name = 'cloudflare'
	`

	rows, err := conn.Query(query)
	if err != nil {
		return err
	}

	for rows.Next() {
		rows.Scan(&c.Enabled)
	}

	return nil
}

func (c *Config) CreateTunnel(hostname string, localPort string) error {
	var localAddress string
	var token string
	var accountId string
	var tunnelId string
	var zoneId string
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
		rows.Scan(&accountId, &tunnelId, &token, &localAddress, &zoneId)
	}

	for _, val := range []struct {
		name, val string
	}{
		{"cloudflare localAddress", localAddress},
		{"cloudflare api token", token},
		{"cloudflare account id", accountId},
		{"cloudflare tunnel id", tunnelId},
		{"cloudflare zone id", zoneId},
	} {
		if val.val == "" {
			return fmt.Errorf("%s is not set", val.name)
		}
	}

	ingress, err := getTunnelIngress(token, accountId, tunnelId)
	if err != nil {
		return err
	}

	alreadyExists := false
	for _, rule := range ingress {
		if rule.Hostname != nil && *rule.Hostname == hostname {
			alreadyExists = true
			break
		}
	}

	if !alreadyExists {
		ingress = append([]ingressRule{
			{
				Hostname: &hostname,
				Service:  "http://" + localAddress + ":" + localPort,
			},
		}, ingress...)
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

	_, err = connection.Api("PUT", url, headers, body)
	if err != nil {
		return fmt.Errorf("failed to update tunnel ingress config: %w", err)
	}

	url = fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneId)
	payload := map[string]interface{}{
		"type":    "CNAME",
		"name":    hostname,
		"content": tunnelId + ".cfargotunnel.com",
		"ttl":     1,
		"proxied": true,
	}

	_, err = connection.Api("POST", url, headers, payload)
	if err != nil {
		return fmt.Errorf("failed to create DNS record: %w", err)
	}

	return nil
}

func getTunnelIngress(token, accountId, tunnelId string) ([]ingressRule, error) {
	url := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations",
		accountId, tunnelId,
	)

	headers := map[string]string{
		"Authorization": "Bearer " + token,
		"Content-Type":  "application/json",
	}

	respBytes, err := connection.Api("GET", url, headers, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tunnel config: %w", err)
	}

	var parsed tunnelConfigResponse
	if err := json.Unmarshal(*respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse tunnel config JSON: %w", err)
	}

	if !parsed.Success {
		return nil, fmt.Errorf("cloudflare API error: %+v", parsed.Errors)
	}

	return parsed.Result.Config.Ingress, nil
}
