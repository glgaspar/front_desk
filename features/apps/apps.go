package apps

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/glgaspar/front_desk/connection"
)

type App struct {
	Command      string  `json:"Command"`
	CreatedAt    string  `json:"CreatedAt"`
	ID           string  `json:"ID"`
	Image        string  `json:"Image"`
	Labels       string  `json:"Labels"`
	LocalVolumes string  `json:"LocalVolumes"`
	Mounts       string  `json:"Mounts"`
	Names        string  `json:"Names"`
	Networks     string  `json:"Networks"`
	Ports        string  `json:"Ports"`
	RunningFor   string  `json:"RunningFor"`
	Size         string  `json:"Size"`
	State        string  `json:"State"`
	Status       string  `json:"Status"`
	Link         *string `json:"Link"`
}

func (a *App) LoadApps() error {
	appList := []App{}
	// docker inspect $(docker ps -q) 
	// is what i probably want, but i dont want to read all that json now
	cmd := exec.Command("sh", "-c", "docker ps --format '{{json .}}' | jq -s .")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	err = json.Unmarshal(output, &appList)
	if err != nil {
		return err
	}

	err = a.UpdateList(&appList)

	return err
}

func (a *App) UpdateList(appList *[]App) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	INSERT INTO apps.list (id, command, createdat, image, labels, localvolumes, mounts, names, networks, ports, runningfor, size, state, status)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	ON CONFLICT (id) DO UPDATE SET
		command = EXCLUDED.command,
		createdat = EXCLUDED.createdat,
		image = EXCLUDED.image,
		labels = EXCLUDED.labels,
		localvolumes = EXCLUDED.localvolumes,
		mounts = EXCLUDED.mounts,
		names = EXCLUDED.names,
		networks = EXCLUDED.networks,
		ports = EXCLUDED.ports,
		runningfor = EXCLUDED.runningfor,
		size = EXCLUDED.size,
		state = EXCLUDED.state,
		status = EXCLUDED.status;
	`

	for i := range *appList {
		_, err := conn.Exec(query,
			(*appList)[i].ID,
			(*appList)[i].Command,
			(*appList)[i].CreatedAt,
			(*appList)[i].Image,
			(*appList)[i].Labels,
			(*appList)[i].LocalVolumes,
			(*appList)[i].Mounts,
			(*appList)[i].Names,
			(*appList)[i].Networks,
			(*appList)[i].Ports,
			(*appList)[i].RunningFor,
			(*appList)[i].Size,
			(*appList)[i].State,
			(*appList)[i].Status,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *App) SetLink(link string) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	query := `
	update apps.list
		set link = $1
	where id = $2 `

	_, err = conn.Exec(query, link, a.ID)
	if err != nil {
		return err
	}
	
	query = `
	select 
		id, command, createdat, image, labels, localvolumes, mounts, names, networks, ports, runningfor, size, state, status, link
	from apps.list
	where id = $1
	`
	rows, err := conn.Query(query, a.ID)
	if err != nil {
		return err
	}

	for rows.Next(){
		rows.Scan(
			&a.ID,
			&a.Command,
			&a.CreatedAt,
			&a.Image,
			&a.Labels,
			&a.LocalVolumes,
			&a.Mounts,
			&a.Names,
			&a.Networks,
			&a.Ports,
			&a.RunningFor,
			&a.Size,
			&a.State,
			&a.Status,
			&a.Link,
		)
	}

	a.Link = &link
	return err
}

func (a *App) GetList() (*[]App, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	
	err = a.LoadApps()
	if err != nil {
		return nil, err
	}
	
	var appList []App
	query := `
	select 
		id, command, createdat, image, labels, localvolumes, mounts, names, networks, ports, runningfor, size, state, status, link
	from apps.list
	`
	rows, err := conn.Query(query)
	for rows.Next(){
		rows.Scan(
			&a.ID,
			&a.Command,
			&a.CreatedAt,
			&a.Image,
			&a.Labels,
			&a.LocalVolumes,
			&a.Mounts,
			&a.Names,
			&a.Networks,
			&a.Ports,
			&a.RunningFor,
			&a.Size,
			&a.State,
			&a.Status,
			&a.Link,
		)
		appList = append(appList, *a)
	}
	return &appList, err
}