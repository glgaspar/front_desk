package models

import (
	"encoding/json"
	"html/template"
	"io"
	"os"
	"time"

	"github.com/labstack/echo/v4"
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

type Templates struct {
	Templates *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.Templates.ExecuteTemplate(w, name, data)
}

type ApiResult struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type PayCheckerBill struct {
	Id          int        `json:"id" db:"id"`
	Description string     `json:"description" db:"description"`
	ExpDay      int        `json:"expDay" db:"expDay"`
	Path        string     `json:"path" db:"path"`
	LastDate    *time.Time `json:"lastDate" db:"lastDate"`
	Track       *bool      `json:"track" db:"track"`
}
