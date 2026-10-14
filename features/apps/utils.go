package apps

import (
	"context"
	"errors"
)

func Rebuild(dir string) (*App, error) {
	path, err := AppPath(dir)
	if err != nil {
		return nil, err
	}

	_, err = runDocker(context.Background(), path, "compose", "up", "-d", "--build")
	if err != nil {
		return nil, err
	}

	containerList, err := inspectCompose(path)
	if err != nil {
		return nil, err
	}

	if len(containerList) != 1 {
		return nil, errors.New("could no retrive data from new container. Please refresh the page")
	}

	newApp := containerList[0].Translate()
	return &newApp, nil

}
