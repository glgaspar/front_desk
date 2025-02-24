package controller

import (
	"front_desk/data"
	"front_desk/models"

	"github.com/labstack/echo/v4"
)

func Root(c echo.Context) error {
	var data = models.RootConfig{}
	if err := data.Generate(); err != nil {
		return c.Render(500, "error", err)
	}
	return c.Render(200, "root", data)
}

func PayChecker(c echo.Context) error {
	data, err := data.GetPayChecker()
	if err != nil {
		return c.Render(500, "error", err)
	}
	return c.Render(200, "paychecker", data)
}
