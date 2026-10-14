package monitoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/lib/pq"
)

type Channel struct {
	Id      int               `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type"` // ntfy | discord | telegram | webhook
	Config  map[string]string `json:"config"`
	Events  []string          `json:"events"`
	Enabled bool              `json:"enabled"`
}

const mask = "********"

// the fields that should never go back to the front
var secrets = map[string][]string{
	"ntfy":     {"token"},
	"discord":  {"url"},
	"telegram": {"token"},
	"webhook":  {"value"},
}

var required = map[string][]string{
	"ntfy":     {"url"},
	"discord":  {"url"},
	"telegram": {"token", "chatId"},
	"webhook":  {"url"},
}

func (c *Channel) Masked() Channel {
	masked := *c
	masked.Config = map[string]string{}
	for k, v := range c.Config {
		masked.Config[k] = v
	}
	for _, key := range secrets[c.Type] {
		if masked.Config[key] != "" {
			masked.Config[key] = mask
		}
	}
	return masked
}

func (c *Channel) validate() error {
	if _, ok := required[c.Type]; !ok {
		return fmt.Errorf("unknown channel type: %s", c.Type)
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required")
	}
	for _, key := range required[c.Type] {
		if c.Config[key] == "" {
			return fmt.Errorf("%s is required for %s", key, c.Type)
		}
	}
	if len(c.Events) == 0 {
		c.Events = []string{"down", "up"}
	}
	return nil
}

func ListChannels() ([]Channel, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(`select id, name, type, config, events, enabled from frontdesk.notification_channels order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels := []Channel{}
	for rows.Next() {
		var ch Channel
		var config []byte
		err = rows.Scan(&ch.Id, &ch.Name, &ch.Type, &config, pq.Array(&ch.Events), &ch.Enabled)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(config, &ch.Config)
		channels = append(channels, ch)
	}
	return channels, nil
}

func GetChannel(id int) (*Channel, error) {
	channels, err := ListChannels()
	if err != nil {
		return nil, err
	}
	for _, ch := range channels {
		if ch.Id == id {
			return &ch, nil
		}
	}
	return nil, fmt.Errorf("channel %d not found", id)
}

func (c *Channel) Create() error {
	err := c.validate()
	if err != nil {
		return err
	}
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	config, _ := json.Marshal(c.Config)
	return conn.QueryRow(`
	insert into frontdesk.notification_channels (name, type, config, events, enabled)
	values ($1, $2, $3, $4, $5) returning id`, c.Name, c.Type, config, pq.Array(c.Events), c.Enabled).Scan(&c.Id)
}

// masked or empty secrets keep whatever was saved before
func (c *Channel) Update() error {
	old, err := GetChannel(c.Id)
	if err != nil {
		return err
	}
	if c.Config == nil {
		c.Config = map[string]string{}
	}
	if old.Type == c.Type {
		for _, key := range secrets[c.Type] {
			if c.Config[key] == "" || c.Config[key] == mask {
				c.Config[key] = old.Config[key]
			}
		}
	}

	err = c.validate()
	if err != nil {
		return err
	}
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	config, _ := json.Marshal(c.Config)
	_, err = conn.Exec(`
	update frontdesk.notification_channels
	set name = $2, type = $3, config = $4, events = $5, enabled = $6
	where id = $1`, c.Id, c.Name, c.Type, config, pq.Array(c.Events), c.Enabled)
	return err
}

func DeleteChannel(id int) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	_, err = conn.Exec(`delete from frontdesk.notification_channels where id = $1`, id)
	return err
}

func (e Event) message() (string, string) {
	switch e.Type {
	case "down":
		msg := e.AppName + " is down"
		if e.Error != "" {
			msg += ": " + e.Error
		}
		return "Front Desk: " + e.AppName + " is down", msg
	case "up":
		return "Front Desk: " + e.AppName + " is back", e.AppName + " is back up"
	}
	return "Front Desk", "Test notification from Front Desk, if you are reading this it works"
}

func post(url string, contentType string, body []byte, headers map[string]string) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		resBody, _ := io.ReadAll(io.LimitReader(res.Body, 500))
		return fmt.Errorf("status: %d - %s", res.StatusCode, string(resBody))
	}
	return nil
}

func (c *Channel) Send(e Event) error {
	title, msg := e.message()
	switch c.Type {
	case "ntfy":
		headers := map[string]string{"Title": title, "Tags": "rotating_light"}
		if e.Type == "up" {
			headers["Tags"] = "white_check_mark"
		}
		if e.Type == "down" {
			headers["Priority"] = "high"
		}
		if c.Config["token"] != "" {
			headers["Authorization"] = "Bearer " + c.Config["token"]
		}
		return post(c.Config["url"], "text/plain", []byte(msg), headers)
	case "discord":
		body, _ := json.Marshal(map[string]string{"content": "**" + title + "**\n" + msg})
		return post(c.Config["url"], "application/json", body, nil)
	case "telegram":
		body, _ := json.Marshal(map[string]string{"chat_id": c.Config["chatId"], "text": title + "\n" + msg})
		return post("https://api.telegram.org/bot"+c.Config["token"]+"/sendMessage", "application/json", body, nil)
	case "webhook":
		body, _ := json.Marshal(map[string]interface{}{"title": title, "message": msg, "event": e})
		headers := map[string]string{}
		if c.Config["header"] != "" {
			headers[c.Config["header"]] = c.Config["value"]
		}
		return post(c.Config["url"], "application/json", body, headers)
	}
	return fmt.Errorf("unknown channel type: %s", c.Type)
}

func (c *Channel) wants(event string) bool {
	for _, e := range c.Events {
		if e == event {
			return true
		}
	}
	return false
}

func Dispatch(e Event) {
	channels, err := ListChannels()
	if err != nil {
		log.Printf("could not load notification channels: %v", err)
		return
	}
	for _, ch := range channels {
		if !ch.Enabled || !ch.wants(e.Type) {
			continue
		}
		go func(ch Channel) {
			err := ch.Send(e)
			if err != nil {
				log.Printf("notification %s (%s) failed: %v", ch.Name, ch.Type, err)
			}
		}(ch)
	}
}

func TestChannel(id int) error {
	ch, err := GetChannel(id)
	if err != nil {
		return err
	}
	return ch.Send(Event{Type: "test", Time: time.Now()})
}
