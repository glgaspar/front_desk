package widgets

import (
	"github.com/glgaspar/front_desk/connection"
)

type Widget struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Position int    `json:"position"`
	Selected bool   `json:"selected"`
}

func (w *Widget) CreateWidget() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	INSERT INTO frontdesk.widgets (name) VALUES ($1)
	returning id, name, enabled, position, selected`

	err = conn.QueryRow(query, w.Name).Scan(&w.Id, 
		&w.Name, 
		&w.Enabled, 
		&w.Position, 
		&w.Selected)

	if err != nil {
		return err
	}
	return nil
}

func (w *Widget) GetList(homeOnly bool) ([]Widget, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	query := `
	SELECT 
		id,
		name,
		enabled,
		position,
		selected
	FROM frontdesk.widgets w
	`
	if homeOnly {
		query = `
		WHERE w.selected = true
		`
	}
	
	query += `ORDER BY position ASC`

	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}

	var widgets []Widget
	for rows.Next() {
		var w Widget
		err := rows.Scan(&w.Id, &w.Name, &w.Enabled, &w.Position, &w.Selected)
		if err != nil {
			return nil, err
		}
		widgets = append(widgets, w)
	}

	return widgets, nil
}

func (w *Widget) Toggle(toggle string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	UPDATE frontdesk.widgets
	`
	
	switch toggle {
	case "enabled":
		query += `
		SET enabled = NOT enabled
		`
	case "selected":
		query += `
		SET selected = NOT selected
		`
	}

	query +=`
	WHERE id = $1
	returning id, name, enabled, position, selected`

	err = conn.QueryRow(query, w.Id).Scan(&w.Id,
		&w.Name,
		&w.Enabled,
		&w.Position,
		&w.Selected)

	return err
}