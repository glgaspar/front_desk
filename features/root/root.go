package root

import (
	"encoding/json"
	"io"
	"os"
)

type RootApp struct {
	Img  string
	Name string
	Path string
}

type RootConfig struct {
	Apps   []RootApp `json:"Apps"`
	Config []RootApp `json:"Config"`
}

func (c *RootConfig) Generate() error {
	file, err := os.Open("routes.json")
	if err != nil {
		return err
	}

	bytesBuffer, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(bytesBuffer, &c); err != nil {
		return err
	}

	return nil
}