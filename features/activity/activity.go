package activity

import (
	"fmt"
	"strings"
	"time"

	"github.com/glgaspar/front_desk/connection"
)

type Entry struct {
	Id        int64     `json:"id"`
	UserId    *int      `json:"userId"`
	UserName  string    `json:"userName"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Status    *int      `json:"status"`
	Ip        string    `json:"ip"`
	CreatedAt time.Time `json:"createdAt"`
}

type Filter struct {
	UserId *int
	Action string
	Before int64
	Limit  int
}

func cut(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

// never makes the request fail, a missing log line is better than a broken button
func Log(e Entry) {
	go func() {
		conn, err := connection.Db()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		_, err = conn.Exec(`
		insert into frontdesk.activity_log (user_id, username, action, target, status, ip)
		values ($1, $2, $3, nullif($4, ''), $5, nullif($6, ''))`,
			e.UserId, cut(e.UserName, 50), cut(e.Action, 100), cut(e.Target, 500), e.Status, cut(e.Ip, 64))
		if err != nil {
			fmt.Println("Error:", err)
		}
	}()
}

func List(f Filter) ([]Entry, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	if f.Limit > 500 {
		f.Limit = 500
	}

	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	query := `
	select id, user_id, coalesce(username, ''), action, coalesce(target, ''), status, coalesce(ip, ''), created_at
	from frontdesk.activity_log
	where ($1::int4 is null or user_id = $1)
	and ($2 = '' or action ilike '%' || $2 || '%' or target ilike '%' || $2 || '%')
	and ($3 = 0 or id < $3)
	order by id desc
	limit $4`
	rows, err := conn.Query(query, f.UserId, strings.TrimSpace(f.Action), f.Before, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Entry{}
	for rows.Next() {
		var e Entry
		err = rows.Scan(&e.Id, &e.UserId, &e.UserName, &e.Action, &e.Target, &e.Status, &e.Ip, &e.CreatedAt)
		if err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, nil
}

func Cleanup(days int) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	_, err = conn.Exec(`delete from frontdesk.activity_log where created_at < now() - make_interval(days => $1)`, days)
	return err
}
