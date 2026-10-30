package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// a cpu only box can take a while to answer, the shared client would give up too soon
var ollamaClient = &http.Client{Timeout: 5 * time.Minute}

func checkOllama(c Config) error {
	if c.Extra == "" {
		return fmt.Errorf("pick a model (llama3.2, qwen2.5...)")
	}
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	err := call(client, "GET", c.Url+"/api/tags", nil, nil, &tags)
	if err != nil {
		return err
	}
	for _, m := range tags.Models {
		if m.Name == c.Extra || strings.TrimSuffix(m.Name, ":latest") == c.Extra {
			return nil
		}
	}
	return fmt.Errorf("model %s is not pulled on ollama", c.Extra)
}

// state is whatever docker says about the container, logs are the last lines of it
func Explain(appName string, image string, state string, logs string) (string, error) {
	c, err := loadEnabled("ollama")
	if err != nil {
		return "", err
	}

	// small models get lost with too much text, the end of the log is what matters anyway
	const maxLogs = 12000
	if len(logs) > maxLogs {
		logs = logs[len(logs)-maxLogs:]
	}

	prompt := fmt.Sprintf(`You are helping someone run docker containers on their home server.
Container: %s
Image: %s
Docker state: %s

Last lines of the container log:
%s

In a few short sentences: is something wrong with this container? If so, what is the most likely cause and how do I fix it? If it looks healthy, say so. Do not repeat the log back.`, appName, image, state, logs)

	return ask(c, prompt)
}

func ask(c *Config, prompt string) (string, error) {
	var res struct {
		Response string `json:"response"`
	}
	body := map[string]interface{}{"model": c.Extra, "prompt": prompt, "stream": false}
	err := call(ollamaClient, "POST", c.Url+"/api/generate", nil, body, &res)
	if err != nil {
		return "", fmt.Errorf("ollama: %w", err)
	}
	return strings.TrimSpace(res.Response), nil
}
