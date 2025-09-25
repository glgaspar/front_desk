package cloudflare

import (
	"fmt"
	"os"

	"github.com/glgaspar/front_desk/connection"
)

type Config struct {
	AccountId          string `json:"accountId" db:"accountId"`
	TunnelId           string `json:"tunnelId" db:"tunnelId"`
	CloudflareAPIToken string `json:"cloudflareAPIToken" db:"cloudflareAPIToken"`
	Hostname           string `json:"hostname" db:"hostname"`
}

func (c *Config) SetCloudflare() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	delete from adm.cloudflare
	insert into adm.cloudflare (accountId, tunnelId, cloudflareAPIToken, hostname)
	values($1,$2,$3,$4)
	`

	_, err = conn.Exec(query, c.AccountId, c.TunnelId, c.CloudflareAPIToken, c.Hostname)
	if err != nil {
		return err
	}

	os.Setenv("CLOUDFLARE", "TRUE")
	os.Setenv("CLOUDFLARE_ACCOUNT_ID", c.AccountId)
	os.Setenv("CLOUDFLARE_TUNNEL_ID", c.TunnelId)
	os.Setenv("CLOUDFLARE_API_TOKEN", c.CloudflareAPIToken)
	os.Setenv("CLOUDFLARE_HOSTNAME", c.Hostname)

	return nil
}

func (c *Config) CheckForCloudflare() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	select accountId, tunnelId, cloudflareAPIToken, hostname 
	from adm.cloudflare
	`

	rows, err := conn.Query(query)
	if err != nil {
		return err
	}

	for rows.Next() {
		rows.Scan(&c.AccountId, &c.TunnelId, &c.CloudflareAPIToken, &c.Hostname)
	}

	if c.AccountId != "" && c.TunnelId != "" && c.CloudflareAPIToken != "" && c.Hostname != "" {
		os.Setenv("CLOUDFLARE", "TRUE")
		os.Setenv("CLOUDFLARE_ACCOUNT_ID", c.AccountId)
		os.Setenv("CLOUDFLARE_TUNNEL_ID", c.TunnelId)
		os.Setenv("CLOUDFLARE_API_TOKEN", c.CloudflareAPIToken)
		os.Setenv("CLOUDFLARE_HOSTNAME", c.Hostname)
	}
	return nil
}

func (c *Config) CreateTunnel() error {
	if os.Getenv("CLOUDFLARE") != "TRUE" {
		return fmt.Errorf("cloudflare is not set up")
	}

	hostname := os.Getenv("CLOUDFLARE_HOSTNAME")
	if hostname == "" {
		return fmt.Errorf("cloudflare hostname is not set")
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

	// for future reference
	// 	curl "https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/cfd_tunnel/$TUNNEL_ID/configurations" \
	//   --request PUT \
	//   --header "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
	//   --json '{
	//     "config": {
	//         "ingress": [
	//             {
	//                 "hostname": "app.example.com",
	//                 "service": "http://localhost:8001",
	//                 "originRequest": {}
	//             },
	//             {
	//                 "service": "http_status:404"
	//             }
	//         ]
	//     }
	//   }'
	// 
	return nil
}
