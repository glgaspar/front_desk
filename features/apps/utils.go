package apps

import (
	"fmt"
	"os"
	"os/exec"
)

func Rebuild(path string) error {
	err := os.Chdir("/src/apps" + path)
	if err != nil {
		return err
	}

	cmd := exec.Command("sh", "-c", "docker compose up -d --build")
	_, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}
	return nil
}

func Prune() error {
	cmd := exec.Command("sh", "-c", "docker system prune")
	_, err := cmd.CombinedOutput()
	return err
}