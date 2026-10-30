package controller

import (
	"net/http"

	"github.com/glgaspar/front_desk/features/monitoring"
	"github.com/labstack/echo/v4"
)

func GetDiskForecast(c echo.Context) error {
	forecast, err := monitoring.GetDiskForecast()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: forecast})
}
