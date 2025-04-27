package timetracker

import (
	"time"

	"github.com/glgaspar/front_desk/connection"
)

type Tracker struct {
	List []time.Time `json:"list"`
}

func (t *Tracker) GetTodayList() error {
	now := time.Now()
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	selectQuery := `
	select dt_entry
	from timetracker.timesheet
	where cast(dt_entry as date) = cast(? as date)
	order by id`
	res, err := conn.Query(selectQuery, now)
	if err != nil {
		return err
	}

	var nt time.Time
	for res.Next() {
		err = res.Scan(&nt)
		if err != nil {
			return err
		}

		t.List = append(t.List, nt)
	}

	return err
}

func (t *Tracker) NewEntry() error {
	now := time.Now()
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()


	insertQuery := `
	insert into timetracker.timesheet (dt_entry)
	values ( ? )
	`
	_, err = conn.Exec(insertQuery,now)
	if err != nil {
		return err
	}


	selectQuery := `
	select dt_entry
	from timetracker.timesheet
	where cast(dt_entry as date) = cast(? as date)
	order by id`
	res, err := conn.Query(selectQuery, now)
	if err != nil {
		return err
	}

	var nt time.Time
	for res.Next() {
		err = res.Scan(&nt)
		if err != nil {
			return err
		}

		t.List = append(t.List, nt)
	}

	return err
}