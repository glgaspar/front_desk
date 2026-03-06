package pihole

import (
	"encoding/json"

	"github.com/glgaspar/front_desk/connection"
)

type Config struct {
	Password string `json:"password"`
	Url      string `json:"url"`
}

func (c *Config) SetPihole() error {
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
	delete from frontdesk.pihole;
	`
	_, err = tran.Exec(query)
	if err != nil {
		tran.Rollback()
		return err
	}

	query = `
	insert into frontdesk.pihole (password, url)
	values ($1, $2);
	`
	_, err = tran.Exec(query, c.Password, c.Url)
	if err != nil {
		tran.Rollback()
		return err
	}

	return tran.Commit()
}

type Pihole struct {
	Url      string `json:"url"`
	Password string `json:"password"`
	Token    string `json:"token"`
	Validity int    `json:"validity"` //in seconds
	Sid      string `json:"sid"`      //session id
	Enabled  bool   `json:"enabled"`
}

func (p *Pihole) Auth() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	select password, url
	from frontdesk.pihole
	`

	rows, err := conn.Query(query)
	if err != nil {
		return err
	}

	for rows.Next() {
		rows.Scan(&p.Password, &p.Url)
	}

	return nil
}

func (p *Pihole) GetHistory() (HistoryWrapper, error) {
	p.Auth()
	var historyWrapper HistoryWrapper

	headers := map[string]string{
		"Authorization": "Bearer " + p.Token,
	}

	res, err := connection.Api("GET", p.Url+"", headers, nil)
	if err != nil {
		return historyWrapper, err
	}

	err = json.Unmarshal(*res, &historyWrapper)
	if err != nil {
		return historyWrapper, err
	}

	return historyWrapper, nil
}

type HistoryWrapper struct {
	History []History `json:"history"`
	Took    float64   `json:"took"`
}

type History struct {
	Timestamp int `json:"timestamp"`
	Total     int `json:"total"`
	Cached    int `json:"cached"`
	Blocked   int `json:"blocked"`
	Forwarded int `json:"forwarded"`
}
