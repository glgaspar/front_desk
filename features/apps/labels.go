package apps

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type ComposeLabels struct {
	Name string
	Dir  string
	Port string
}

type composeFile struct {
	Services map[string]struct {
		Labels interface{} `yaml:"labels"`
	} `yaml:"services"`
}

// labels can come as a map or as a list of "key=value", compose accepts both so we do too
func ParseComposeLabels(compose string) (ComposeLabels, error) {
	var labels ComposeLabels
	var file composeFile

	err := yaml.Unmarshal([]byte(compose), &file)
	if err != nil {
		return labels, fmt.Errorf("invalid compose file: %v", err)
	}

	for _, service := range file.Services {
		values := map[string]string{}
		switch l := service.Labels.(type) {
		case map[string]interface{}:
			for key, val := range l {
				if val != nil {
					values[key] = strings.TrimSpace(fmt.Sprint(val))
				}
			}
		case []interface{}:
			for _, item := range l {
				key, val, _ := strings.Cut(fmt.Sprint(item), "=")
				values[strings.TrimSpace(key)] = strings.TrimSpace(val)
			}
		}

		if values["front-desk.name"] == "" && values["front-desk.dir"] == "" {
			continue
		}

		labels.Name = values["front-desk.name"]
		labels.Dir = values["front-desk.dir"]
		labels.Port = values["front-desk.port"]
		if labels.Port == "" {
			labels.Port = values["front-desk.cloudflare-port"]
		}
		break
	}

	if labels.Name == "" {
		return labels, fmt.Errorf("front-desk.name label is missing")
	}
	if labels.Dir == "" {
		return labels, fmt.Errorf("front-desk.dir label is missing")
	}
	if labels.Dir[0] != '/' {
		labels.Dir = "/" + labels.Dir
	}

	_, err = AppPath(labels.Dir)
	if err != nil {
		return labels, err
	}

	return labels, nil
}
