package integrations

import (
	"log"

	"github.com/glgaspar/front_desk/connection"
)

func SetAvailable(name string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()
	query := `
	update frontdesk.integrations_available
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
	update frontdesk.integrations_available
	set available = FALSE
	where name = $1
	`
	_, err = conn.Exec(query, name)
	if err != nil {
		return err
	}
	return nil
}

func CheckFor(integration string) (bool, error) {
	enabled := false
	conn, err := connection.Db()
	if err != nil {
		return enabled, err
	}
	defer conn.Close()

	query := `
	select enabled
	from frontdesk.integrations_available
	where name = $1;
	`

	rows, err := conn.Query(query, integration)
	if err != nil {
		return enabled, err
	}

	for rows.Next() {
		rows.Scan(&enabled)
	}

	return enabled, nil
}

func CheckAll() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()
	query := `
	select name
	from frontdesk.integrations_available;
	`

	rows, err := conn.Query(query)
	if err != nil {
		return err
	}

	var integrations []string
	for rows.Next() {
		var integration string
		rows.Scan(&integration)
		integrations = append(integrations, integration)
	}

	redBg := "\033[41m"
	greenBg := "\033[42m"
	reset := "\033[0m"

	for _, integration := range integrations {
		log.Println("checking for " + integration + "... ")

		enabled, err := CheckFor(integration)
		if err != nil {
			log.Printf("%sFAILED%s: %v", redBg, reset, err)
			return err
		}

		if enabled {
			log.Println(integration + " available")
		} else {
			log.Println(integration + " not available")
		}

		log.Printf("%sOK%s", greenBg, reset)
	}
	return nil
}
