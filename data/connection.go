package data

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func Api(method string, data any, token *string, host string) (*[]byte, error) {
	payloadBuffer, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	payload := bytes.NewBuffer(payloadBuffer)
	req, err := http.NewRequest(method, host, payload)
	if err != nil {
		return nil, err
	}
	if token != nil {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s",*token))
	}

	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, errors.New(string(body))
	}

	return &body, nil
}