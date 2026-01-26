package integrations

import (
	"github.com/glgaspar/front_desk/connection"
	"log"
)

func SetAvailable(name string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()
	query := `
	update adm.integrations
	set available = TRUE
	where name = $1
	`
	_, err = conn.Exec(query, name)
	if err != nil {
		return err
	}
	return nil
}

func SetUnavailable(name string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()
	query := `
	update adm.integrations
	set available = FALSE
	where name = $1
	`
	_, err = conn.Exec(query, name)
	if err != nil {
		return err
	}
	return nil
}

func CheckFor(integration string) error {
	enabled := false
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	select enabled
	from adm.integrations_available
	where name = $1;
	`

	rows, err := conn.Query(query, integration)
	if err != nil {
		return err
	}

	for rows.Next() {
		rows.Scan(&enabled)
	}

	if enabled {
		log.Println(integration + " available")
	} else {
		log.Println(integration + " not available")
	}
	return nil
}
