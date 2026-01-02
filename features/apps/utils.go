package apps

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func Rebuild(path string) (*App, error) {
	os.Chdir("/mnt/apps" + path)
	cmd := exec.Command("sh", "-c", "docker compose up -d --build")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("\n%s", cmd.Stdout)
	}

	var containerList []Container
	cmd = exec.Command("sh", "-c", "docker inspect $(docker compose ps -q)")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("\n%s", exitErr.Stderr)
		}
		return nil, err
	}

	err = json.Unmarshal(output, &containerList)
	if err != nil {
		return nil, err
	}

	if len(containerList) != 1 {
		return nil, errors.New("could no retrive data from new container. Please refresh the page")
	}

	newApp := containerList[0].Translate()
	return &newApp, nil

}

func Prune() error {
	cmd := exec.Command("sh", "-c", "docker system prune")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("\n%s", cmd.Stdout)
	}
	return nil
}
