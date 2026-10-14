package apps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glgaspar/front_desk/features/integrations"
	"github.com/glgaspar/front_desk/features/integrations/cloudflare"
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
		Image  string `json:"Image"`
		Labels struct {
			Name string  `json:"front-desk.name"`
			Url  string  `json:"front-desk.url"`
			Dir  string  `json:"front-desk.dir"`
			Logo *string `json:"front-desk.logo"`
			Port           *string `json:"front-desk.port"`
			CloudflarePort *string `json:"front-desk.cloudflare-port"`
		} `json:"Labels"`
	} `json:"Config"`
}

type Compose struct {
	Compose string `json:"compose"`
	Tunnel  *bool  `json:"tunnel"`
}

func (c *Container) Translate() App {
	return App{
		Id:             c.ID,
		Created:        c.Created,
		Image:          c.Image,
		ImageRef:       c.Config.Image,
		Port:           c.Config.Labels.Port,
		CloudflarePort: c.Config.Labels.CloudflarePort,
		Name:           c.Config.Labels.Name,
		Url:            c.Config.Labels.Url,
		Dir:            c.Config.Labels.Dir,
		Logo:           c.Config.Labels.Logo,
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
	if !ValidID(a.ID) {
		return nil, fmt.Errorf("invalid id: %s", a.ID)
	}

	containerList, err := inspect(a.ID)
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

	path, err := AppPath(app.Dir)
	if err != nil {
		return compose, err
	}

	file, err := os.ReadFile(filepath.Join(path, "docker-compose.yml"))
	if err != nil {
		return compose, err
	}
	compose = string(file)
	return compose, nil
}

func (c *Container) SaveCompose(compose Compose) (*App, error) {
	app, err := c.GetApp()
	if err != nil {
		return nil, err
	}
	path, err := AppPath(app.Dir)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(filepath.Join(path, "docker-compose.yml"), []byte(compose.Compose), 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return nil, err
	}

	return Rebuild(app.Dir)
}

// the string is a warning, the app is gone but something on the side didnt work
func (c *Container) RemoveContainer() (string, error) {
	app, err := c.GetApp()
	if err != nil {
		return "", err
	}
	path, err := AppPath(app.Dir)
	if err != nil {
		return "", err
	}

	warning := ""
	cloudflareEnabled, _ := integrations.CheckFor("cloudflare")
	if cloudflareEnabled {
		err = cloudflare.RemoveForApp(app.Dir, cloudflare.HostFromUrl(app.Url), app.ForwardPort())
		if err != nil {
			fmt.Println("Error:", err)
			warning = "Could not clean cloudflare: " + err.Error()
		}
	}

	// only the images this app built. prune used to clean the whole server, oops
	_, err = runDocker(context.Background(), path, "compose", "down", "--rmi", "local")
	if err != nil {
		return warning, err
	}

	err = os.RemoveAll(path)
	if err != nil {
		return warning, err
	}

	return warning, nil
}
