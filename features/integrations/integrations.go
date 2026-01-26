package integrations

import "github.com/glgaspar/front_desk/connection"

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