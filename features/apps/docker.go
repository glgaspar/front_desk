package apps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var validId = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func ValidID(id string) bool {
	return validId.MatchString(id)
}

func AppsDir() string {
	dir := os.Getenv("APPS_DIR")
	if dir == "" {
		return "/src/apps"
	}
	return filepath.Clean(dir)
}

// full path inside the apps folder. never lets you out of it and never returns the folder itself
// because RemoveAll on that would be a very bad day
func AppPath(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("this app has no path set")
	}

	base := AppsDir()
	path := filepath.Join(base, filepath.Clean("/"+dir))
	if path == base || !strings.HasPrefix(path, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid app dir: %s", dir)
	}

	return path, nil
}

func runDocker(ctx context.Context, dir string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("\n%s%s", stderr.String(), stdout.String())
	}

	return stdout.Bytes(), nil
}

func inspect(ids ...string) ([]Container, error) {
	var containerList []Container
	if len(ids) == 0 {
		return containerList, nil
	}

	output, err := runDocker(context.Background(), "", append([]string{"inspect"}, ids...)...)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(output, &containerList)
	if err != nil {
		return nil, err
	}

	return containerList, nil
}

func composeIDs(path string) ([]string, error) {
	output, err := runDocker(context.Background(), path, "compose", "ps", "-q")
	if err != nil {
		return nil, err
	}

	return strings.Fields(string(output)), nil
}

func inspectCompose(path string) ([]Container, error) {
	ids, err := composeIDs(path)
	if err != nil {
		return nil, err
	}
	return inspect(ids...)
}
