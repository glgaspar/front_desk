package apps

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
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
}


func (a *App) GetList() ([]App, error) {
	var appList []App
	cmd := exec.Command("sh", "-c", "docker inspect $(docker ps -a -q)")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return appList, err
	}

	err = json.Unmarshal(output, &appList)
	if err != nil {
		return appList, err
	}
	
	return appList, nil 
}

func (a *App) ToggleOnOFF(id string, toggle string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("docker %s %s", toggle, id))
	_, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	cmd = exec.Command("sh", "-c", fmt.Sprintf("docker inspect %s",id))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	err = json.Unmarshal(output, &a)
	if err != nil {
		return err
	}

	return nil
}

