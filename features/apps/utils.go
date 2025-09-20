package apps

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func Rebuild(path string) (*App, error) {
	os.Chdir("/mnt/apps"+path)
	cmd := exec.Command("sh", "-c", "docker compose up -d --build")
	_, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}

	var containerList []Container
	cmd = exec.Command("sh", "-c", "docker inspect $(docker compose ps -q)")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
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
	return err
}