package apps

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/glgaspar/front_desk/features/utils/messenger"

	"github.com/glgaspar/front_desk/features/integrations/cloudflare"
)

type BuildLogMsg struct {
	App  string `json:"app"`
	Log  string `json:"log"`
	Time int64  `json:"time"`
	Done bool   `json:"done"`
}

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
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if strings.Contains(string(exitErr.Stderr), "requires at least 1 argument") {
				return appList, nil
			}
			return appList, fmt.Errorf("\n%s", exitErr.Stderr)
		}
		fmt.Println("Error:", err)
		return appList, err
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return fmt.Errorf("\n%s", output)
	}

	container := Container{ID: id}
	app, err := container.GetApp()
	if err != nil {
		return err
	}
	*a = *app
	return nil

}

func (a *App) sendBuildLog(appName string, message string) error {
	b, _ := json.Marshal(map[string]interface{}{
		"app":  appName,
		"log":  message,
		"time": time.Now().UnixNano(),
		"done": false,
	})
	return messenger.SendMessage("build-logs", string(b))
}

func (a *App) CreateApp(compose Compose, appName string, dir string) error {
	err := os.MkdirAll("/src/apps"+dir, 0777)
	if err != nil {
		return err
	}

	err = a.sendBuildLog(appName, "Directory created.")
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

	err = a.sendBuildLog(appName, "Initializing build.")
	if err != nil {
		return err
	}

	newApp, err := Rebuild(dir)
	if err != nil {
		return err
	}
	*a = *newApp

	err = a.sendBuildLog(appName, "Build finished.")
	if err != nil {
		return err
	}

	container, err := a.GetContainer()
	if err != nil {
		return err
	}

	if (*container).Config.Labels.Port == nil {
		return a.sendBuildLog(appName, "App was created but no port provided for tunnel.")
	}

	if compose.Tunnel != nil && *compose.Tunnel {
		err = a.sendBuildLog(appName, "Creating Cloudflare tunnel.")
		if err != nil {
			return err
		}

		cloudflareConfig := new(cloudflare.Config)
		err = cloudflareConfig.CreateTunnel(strings.Replace(a.Url, "https://", "", 1), *container.Config.Labels.Port)
		if err != nil {
			return err
		}
		err = a.sendBuildLog(appName, "Tunnel Cloudflare created.")
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
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("\n%s", exitErr.Stderr)
		}
		return nil, err
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
	cmd := exec.Command("docker", "logs", "-f", a.Id)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating StdoutPipe: %v", err)
	}

	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	go func(scanner *bufio.Scanner, channel *chan string) {
		for scanner.Scan() {
			line := scanner.Text()
			*channel <- line
		}
	}(scanner, channel)

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
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("\n%s", exitErr.Stderr)
		}
		return err
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
