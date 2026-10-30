package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/glgaspar/front_desk/features/integrations/services"
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

func GetSeerrPending(c echo.Context) error {
	pending, err := services.SeerrPendingRequests()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d requests waiting", len(pending)), Data: pending})
}

func AnswerSeerrRequest(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	action := c.Param("action")
	if err != nil || (action != "approve" && action != "decline") {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Both id (int) and action (\"approve\", \"decline\") must be sent"})
	}

	err = services.SeerrAnswer(id, action)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetMediaCalendar(c echo.Context) error {
	days, err := strconv.Atoi(c.QueryParam("days"))
	if err != nil || days < 1 || days > 31 {
		days = 7
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: services.MediaCalendar(days)})
}
