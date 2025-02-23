package controller

import (
	"front_desk/models"
	"front_desk/data"
	"github.com/labstack/echo/v4"
)


func Root(c echo.Context) error {
	var data = models.RootConfig{}
	data.Generate()
	return c.Render(200, "index", data)
}

func PayChecker(c echo.Context) error {
	data,err := data.GetPayChecker()
	if err != nil {
		return c.Render(500, "error", err.Error())
	}
	return c.Render(200, "paychecker", data)
}