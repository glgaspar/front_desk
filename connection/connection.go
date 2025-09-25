package connection

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

func Db() (*sql.DB, error) {
	HOST := os.Getenv("PG_HOST")
	PORT := os.Getenv("PG_PORT")
	USER := os.Getenv("PG_USER")
	PASSWORD := os.Getenv("PG_PASSWORD")
	DBNAME := os.Getenv("PG_DBNAME")

	conn, err := sql.Open("postgres",
		fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			HOST, PORT, USER, PASSWORD, DBNAME),
	)

	if err != nil {
		return nil, err
	}
	err = conn.Ping()

	return conn, err
}

func Api(method string, url string, headers map[string]string, data any) (*[]byte, error) {
	payload := &bytes.Buffer{}
	if data != nil {
		payloadBuffer, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewBuffer(payloadBuffer)
	}
	
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Add(key,value)
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	
	if response.StatusCode < 200 || response.StatusCode > 226 { // just in case any of you go full semantic on returns. Accepting all 200s in http cats
		return nil, errors.New(fmt.Sprintf("status: %d - ", response.StatusCode) + string(body))
	}
	
	return &body, nil
}