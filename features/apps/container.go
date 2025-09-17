package apps

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type Container struct {
	ID      string    `json:"Id"`
	Created time.Time `json:"Created"`
	State   struct {
		Status     string    `json:"Status"`
		ExitCode   int       `json:"ExitCode"`
		Error      string    `json:"Error"`
		StartedAt  time.Time `json:"StartedAt"`
		FinishedAt time.Time `json:"FinishedAt"`
	} `json:"State"`
	Image        string `json:"Image"`
	Name         string `json:"Name"`
	RestartCount int    `json:"RestartCount"`
	Config       struct {
		Labels struct {
			Name string  `json:"front-desk.name"`
			Url  string  `json:"front-desk.url"`
			Dir  string  `json:"front-desk.dir"`
			Logo *string `json:"front-desk.logo"`
		} `json:"Labels"`
	} `json:"Config"`
}

type Compose struct {
	Compose string `json:"compose"`
}

func (c *Container) Translate() App {
	return App{
		Id:      c.ID,
		Created: c.Created,
		Image:   c.Image,
		Name:    c.Config.Labels.Name,
		Url:     c.Config.Labels.Url,
		Dir:     c.Config.Labels.Dir,
		Logo:    c.Config.Labels.Logo,
		State: struct {
			Status     string    `json:"status"`
			ExitCode   int       `json:"exitCode"`
			Error      string    `json:"error"`
			StartedAt  time.Time `json:"startedAt"`
			FinishedAt time.Time `json:"finishedAt"`
		}{
			Status:     c.State.Status,
			ExitCode:   c.State.ExitCode,
			Error:      c.State.Error,
			StartedAt:  c.State.StartedAt,
			FinishedAt: c.State.FinishedAt,
		},
	}
}

func (a *Container) GetApp() (*App, error) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("docker inspect %s", a.ID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}
	var containerList []Container

	err = json.Unmarshal(output, &containerList)
	if err != nil {
		return nil, err
	}

	if len(containerList) == 1 {
		app := containerList[0].Translate()
		return &app, nil
	}

	return nil, fmt.Errorf("%d containers returned for that ID", len(containerList))
}

func (c *Container) GetCompose() (string, error) {
	var compose string = ""
	app, err := c.GetApp()
	if err != nil {
		return compose, err
	}

	path, err := app.GetPath()
	if err != nil {
		return compose, err
	}

	file, err := os.ReadFile("/src/apps" + path + "/docker-compose.yml")
	if err != nil {
		return compose, err
	}
	compose = string(file)
	return compose, nil
}

func (c *Container) SaveCompose(compose Compose) error {
	app, err := c.GetApp()
	if err != nil {
		return err
	}
	path, err := app.GetPath()
	if err != nil {
		return err
	}
	err = os.WriteFile("/src/apps"+path+"/docker-compose.yml", []byte(compose.Compose), 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
	}

	return Rebuild(path)
}

func (c *Container) Delete() error {
	app, err := c.GetApp()
	if err != nil {
		return err
	}
	path, err := app.GetPath()
	if err != nil {
		return err
	}

	err = os.Chdir("/src/apps" + path)
	if err != nil {
		return err
	}
	cmd := exec.Command("sh", "-c", "docker compose down --rmi all")
	_, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	err = Prune()
	if err != nil {
		return err
	}

	err = os.RemoveAll("/src/apps" + path)
	if err != nil {
		return err
	}

	return nil
}
