package apps

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/glgaspar/front_desk/connection"
)

type App struct {
	ID      string        `db:"id" json:"Id"`
	Created time.Time     `db:"created" json:"Created"`
	State   struct {
		Status     string    `db:"status" json:"Status"`
		ExitCode   int       `db:"exitcode" json:"ExitCode"`
		Error      string    `db:"error" json:"Error"`
		StartedAt  time.Time `db:"startedat" json:"StartedAt"`
		FinishedAt time.Time `db:"finishedat" json:"FinishedAt"`
	} `db:"state" json:"State"`
	Image           string      `db:"image" json:"Image"`
	Name            string      `db:"name" json:"Name"`
	RestartCount    int         `db:"restartcount" json:"RestartCount"`
	Config struct {
		Labels     struct {
			Project            	string `db:"project" json:"com.docker.compose.project"`
			ConfigFiles 		string `db:"configfiles" json:"com.docker.compose.project.config_files"`
			WorkingDir  		string `db:"workingdir" json:"com.docker.compose.project.working_dir"`
			Replace            	string `db:"replace" json:"com.docker.compose.replace"`
		} `db:"labels" json:"Labels"`
	} `db:"config" json:"Config"`
	Link *string `db:"link" json:"link"`
}


func (a *App) LoadApps() error {
	var appList []App
	cmd := exec.Command("sh", "-c", "docker inspect $(docker ps -q)")
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
	INSERT INTO apps.list (id,created,status,exitcode,error,startedat,finishedat,image,name,restartcount,project,configfiles,workingdir,replace)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
	
	ON CONFLICT (id) DO UPDATE SET
		created = EXCLUDED.created,
		status = EXCLUDED.status,
		exitcode = EXCLUDED.exitcode,
		error = EXCLUDED.error,
		startedat = EXCLUDED.startedat,
		finishedat = EXCLUDED.finishedat,
		image = EXCLUDED.image,
		name = EXCLUDED.name,
		restartcount = EXCLUDED.restartcount,
		project = EXCLUDED.project,
		configfiles = EXCLUDED.configfiles,
		workingdir = EXCLUDED.workingdir,
		replace = EXCLUDED.replace;
	`

	for i := range *appList {
		_, err := conn.Exec(query,
			(*appList)[i].ID,
			(*appList)[i].Created,
			(*appList)[i].State.Status,
			(*appList)[i].State.ExitCode,
			(*appList)[i].State.Error,
			(*appList)[i].State.StartedAt,
			(*appList)[i].State.FinishedAt,
			(*appList)[i].Image,
			(*appList)[i].Name,
			(*appList)[i].RestartCount,
			(*appList)[i].Config.Labels.Project,
			(*appList)[i].Config.Labels.ConfigFiles,
			(*appList)[i].Config.Labels.WorkingDir,
			(*appList)[i].Config.Labels.Replace,
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
		id,created,status,exitcode,error,startedat,finishedat,image,name,restartcount,project,configfiles,workingdir,replace,link
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
			&a.Created,
			&a.State.Status,
			&a.State.ExitCode,
			&a.State.Error,
			&a.State.StartedAt,
			&a.State.FinishedAt,
			&a.Image,
			&a.Name,
			&a.RestartCount,
			&a.Config.Labels.Project,
			&a.Config.Labels.ConfigFiles,
			&a.Config.Labels.WorkingDir,
			&a.Config.Labels.Replace,
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
		id,
		created,
		status,
		exitcode,
		error,
		startedat,
		finishedat,
		image,
		name,
		restartcount,
		labels,
		project,
		configfiles,
		workingdir,
		replace,
		link
	from apps.list
	`
	rows, err := conn.Query(query)
	for rows.Next(){
		var app App
		rows.Scan(
			&app.ID,
			&app.Created,
			&app.State.Status,
			&app.State.ExitCode,
			&app.State.Error,
			&app.State.StartedAt,
			&app.State.FinishedAt,
			&app.Image,
			&app.Name,
			&app.RestartCount,
			&app.Config.Labels.Project,
			&app.Config.Labels.ConfigFiles,
			&app.Config.Labels.WorkingDir,
			&app.Config.Labels.Replace,
			&app.Link,
		)
		appList = append(appList, app)
	}
	return &appList, err
}