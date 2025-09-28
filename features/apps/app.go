package apps

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/glgaspar/front_desk/features/cloudflare"
)

type App struct {
	Id      string    `json:"id"`
	Created time.Time `json:"created"`
	Image   string    `json:"image"`
	Name    string    `json:"name"`
	Url     string    `json:"url"`
	Dir     string    `json:"dir"`
	Logo    *string   `json:"logo"`
	State   struct {
		Status     string    `json:"status"`
		ExitCode   int       `json:"exitCode"`
		Error      string    `json:"error"`
		StartedAt  time.Time `json:"startedAt"`
		FinishedAt time.Time `json:"finishedAt"`
	} `json:"state"`
}

func (a *App) GetList() ([]App, error) {
	var appList []App
	var containerList []Container
	cmd := exec.Command("sh", "-c", "docker inspect $(docker ps -a -q)")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return appList, fmt.Errorf("\n%s", cmd.Stdout)
	}

	err = json.Unmarshal(output, &containerList)
	if err != nil {
		return appList, err
	}

	for _, container := range containerList {
		appList = append(appList, container.Translate())
	}

	return appList, nil
}

func (a *App) ToggleOnOFF(id string, toggle string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("docker %s %s", toggle, id))
	_, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return fmt.Errorf("\n%s", cmd.Stdout)
	}

	container := Container{ID: id}
	app, err := container.GetApp()
	if err != nil {
		return err
	}
	*a = *app
	return nil

}

func (a *App) CreateApp(compose Compose) error {
	var dir string
	reader := strings.NewReader(compose.Compose)

	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "front-desk.dir:") {
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid front-desk.dir label")
			}
			dir = strings.TrimSpace(parts[1])
			if dir[0] != '/' {
				dir = "/" + dir
			}
		}
	}

	err := os.Mkdir("/src/apps"+dir, 0777)
	if err != nil {
		return err
	}

	err = os.Chdir("/src/apps" + dir)
	if err != nil {
		return err
	}

	err = os.WriteFile("docker-compose.yml", []byte(compose.Compose), 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return err
	}
	newApp, err := Rebuild(dir)
	if err != nil {
		return err
	}
	*a = *newApp

	container, err := a.GetContainer()
	if err != nil {
		return err
	}

	if (*container).Config.Labels.Port == nil {
		return fmt.Errorf("App was created but no port provided to for tunnel")
	}

	if compose.Tunnel != nil && *compose.Tunnel {
		cloudflareConfig := new(cloudflare.Config)
		err = cloudflareConfig.CreateTunnel(strings.Replace(a.Url, "https://", "", 1), *container.Config.Labels.Port)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *App) GetPath() (string, error) {
	if a.Dir == "" {
		return "", fmt.Errorf("this app has no path set")
	}
	if a.Dir[0] != '/' {
		return "/" + a.Dir, nil
	}

	return a.Dir, nil
}

func (a *App) GetContainer() (*Container, error) {
	err := os.Chdir("/src/apps" + a.Dir)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("sh", "-c", "docker inspect $(docker compose ps -q)")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("\n%s", cmd.Stdout)
	}

	var containerList []Container

	err = json.Unmarshal(output, &containerList)
	if err != nil {
		return nil, err
	}

	if len(containerList) == 1 {
		return &containerList[0], nil
	}

	return nil, fmt.Errorf("%d containers returned for that ID", len(containerList))
}

func (a *App) GetLogs(channel *chan string) error {
	err := os.Chdir("/src/apps" + a.Dir)
	if err != nil {
		return err
	}

	cmd := exec.Command("docker", "compose", "logs", "-f")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating StdoutPipe: %v", err)
	}

	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			*channel <- line
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command finished with error: %v", err)
	}
	return nil
}

func (a *App) GetApp() error {
	if a.Id == "" && a.Dir == "" {
		return fmt.Errorf("either id or dir must be provided to reach app")
	}
	var command string
	if a.Dir != "" {
		err := os.Chdir("/src/apps" + a.Dir)
		if err != nil {
			return err
		}
		command = "docker inspect $(docker compose ps -q)"
	} else {
		command = "docker inspect " + a.Id
	}

	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return fmt.Errorf("\n%s", cmd.Stdout)
	}
	var containerList []Container

	err = json.Unmarshal(output, &containerList)
	if err != nil {
		return err
	}

	if len(containerList) == 1 {
		app := containerList[0].Translate()
		*a = app
		return nil
	}

	return fmt.Errorf("%d containers returned for that ID", len(containerList))
}
