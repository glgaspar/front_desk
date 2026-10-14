package apps

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/glgaspar/front_desk/features/integrations"
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
	Id             string    `json:"id"`
	Created        time.Time `json:"created"`
	Image          string    `json:"image"`
	ImageRef       string    `json:"imageRef"`
	Port           *string   `json:"port"`
	CloudflarePort *string   `json:"cloudflarePort"`
	Name           string    `json:"name"`
	Url            string    `json:"url"`
	Dir            string    `json:"dir"`
	Logo           *string   `json:"logo"`
	State          struct {
		Status     string    `json:"status"`
		ExitCode   int       `json:"exitCode"`
		Error      string    `json:"error"`
		StartedAt  time.Time `json:"startedAt"`
		FinishedAt time.Time `json:"finishedAt"`
	} `json:"state"`
}

func (a *App) GetList() ([]App, error) {
	var appList []App
	output, err := runDocker(context.Background(), "", "ps", "-a", "-q")
	if err != nil {
		return appList, err
	}

	containerList, err := inspect(strings.Fields(string(output))...)
	if err != nil {
		return appList, err
	}

	for _, container := range containerList {
		appList = append(appList, container.Translate())
	}

	return appList, nil
}

func (a *App) ToggleOnOFF(id string, toggle string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid id: %s", id)
	}
	if toggle != "start" && toggle != "stop" {
		return fmt.Errorf("invalid toggle: %s", toggle)
	}

	_, err := runDocker(context.Background(), "", toggle, id)
	if err != nil {
		return err
	}

	container := Container{ID: id}
	app, err := container.GetApp()
	if err != nil {
		return err
	}
	*a = *app
	return nil

}

func SendBuildLog(appName string, message string) error {
	return sendBuild(appName, message, false)
}

func SendBuildDone(appName string, message string) error {
	return sendBuild(appName, message, true)
}

func sendBuild(appName string, message string, done bool) error {
	b, _ := json.Marshal(BuildLogMsg{
		App:  appName,
		Log:  message,
		Time: time.Now().UnixNano(),
		Done: done,
	})
	return messenger.SendMessage("build-logs", string(b))
}

func (a *App) CreateApp(compose Compose, appName string, dir string, tunnel bool) error {
	path, err := AppPath(dir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(path, 0777)
	if err != nil {
		return err
	}

	err = SendBuildLog(appName, "Directory created.")
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath.Join(path, "docker-compose.yml"), []byte(compose.Compose), 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return err
	}

	err = SendBuildLog(appName, "Initializing build.")
	if err != nil {
		return err
	}

	newApp, err := Rebuild(dir)
	if err != nil {
		return err
	}
	a = newApp

	err = SendBuildLog(appName, "Build finished.")
	if err != nil {
		return err
	}

	cloudflareEnabled, err := integrations.CheckFor("cloudflare")
	if err != nil {
		return err
	}

	if cloudflareEnabled && tunnel {
		if a.ForwardPort() == "" {
			return SendBuildLog(appName, "App was created but no port provided for tunnel.")
		}

		err = SendBuildLog(appName, "Creating Cloudflare tunnel.")
		if err != nil {
			return err
		}

		err = cloudflare.AddHostname(cloudflare.HostFromUrl(a.Url), a.ForwardPort(), a.Dir)
		if err != nil {
			return err
		}
		err = SendBuildLog(appName, "Tunnel Cloudflare created.")
		if err != nil {
			return err
		}
	}

	return nil
}

// front-desk.port is the one to use, cloudflare-port is still here so old apps keep working
func (a *App) ForwardPort() string {
	if a.Port != nil && *a.Port != "" {
		return *a.Port
	}
	if a.CloudflarePort != nil {
		return *a.CloudflarePort
	}
	return ""
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
	path, err := AppPath(a.Dir)
	if err != nil {
		return nil, err
	}

	containerList, err := inspectCompose(path)
	if err != nil {
		return nil, err
	}

	if len(containerList) == 1 {
		return &containerList[0], nil
	}

	return nil, fmt.Errorf("%d containers returned for that ID", len(containerList))
}

// dies with the context, so when the browser leaves we stop following the logs
func (a *App) GetLogs(ctx context.Context, onLine func(string)) error {
	if !ValidID(a.Id) {
		return fmt.Errorf("invalid id: %s", a.Id)
	}

	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--tail", "500", a.Id)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating StdoutPipe: %v", err)
	}

	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		onLine(scanner.Text())
	}

	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("command finished with error: %v", err)
	}
	return nil
}

func (a *App) GetApp() error {
	if a.Id == "" && a.Dir == "" {
		return fmt.Errorf("either id or dir must be provided to reach app")
	}

	var containerList []Container
	var err error
	if a.Dir != "" {
		path, err := AppPath(a.Dir)
		if err != nil {
			return err
		}
		containerList, err = inspectCompose(path)
		if err != nil {
			return err
		}
	} else {
		if !ValidID(a.Id) {
			return fmt.Errorf("invalid id: %s", a.Id)
		}
		containerList, err = inspect(a.Id)
		if err != nil {
			return err
		}
	}

	if len(containerList) == 1 {
		app := containerList[0].Translate()
		*a = app
		return nil
	}

	return fmt.Errorf("%d containers returned for that ID", len(containerList))
}
