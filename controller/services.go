package controller

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/glgaspar/front_desk/features/integrations"
	"github.com/glgaspar/front_desk/features/integrations/services"
	"github.com/labstack/echo/v4"
)

func SetService(c echo.Context) error {
	name := c.Param("name")
	if !services.Known(name) {
		return c.JSON(http.StatusNotFound, Response{Status: false, Message: "Unknown service " + name})
	}

	var data services.Config
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = services.Save(name, data)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetService(c echo.Context) error {
	name := c.Param("name")
	if !services.Known(name) {
		return c.JSON(http.StatusNotFound, Response{Status: false, Message: "Unknown service " + name})
	}

	enabled, err := integrations.CheckFor(name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	config, err := services.Public(name)
	if err != nil {
		return c.JSON(http.StatusOK, Response{Status: false, Message: name + " not configured"})
	}

	return c.JSON(http.StatusOK, Response{Status: enabled, Message: "Operation successful", Data: config})
}

func PiholeHistory(c echo.Context) error {
	history, err := services.PiholeHistory()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: history})
}
