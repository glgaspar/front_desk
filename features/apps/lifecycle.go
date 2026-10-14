package apps

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func (a *App) Restart(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid id: %s", id)
	}

	_, err := runDocker(context.Background(), "", "restart", id)
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

// same as runDocker but every line goes to the build log so the front can follow along
func RunStreaming(ctx context.Context, path string, appName string, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = path

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	var last string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		last = scanner.Text()
		SendBuildLog(appName, last)
	}

	if err := cmd.Wait(); err != nil {
		fmt.Println("Error:", err)
		return fmt.Errorf("docker %s failed: %s", strings.Join(args, " "), last)
	}
	return nil
}

// pull the new images and recreate whatever changed
func (a *App) Update() error {
	path, err := AppPath(a.Dir)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	SendBuildLog(a.Name, "Pulling images.")
	err = RunStreaming(ctx, path, a.Name, "compose", "pull", "--ignore-buildable")
	if err != nil {
		return err
	}

	SendBuildLog(a.Name, "Recreating containers.")
	err = RunStreaming(ctx, path, a.Name, "compose", "up", "-d", "--remove-orphans")
	if err != nil {
		return err
	}

	ForgetUpdate(a.ImageRef)
	return nil
}
