package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

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

func GetPiholeDns(c echo.Context) error {
	records, err := services.ListDns()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d records found", len(records)), Data: map[string]interface{}{
		"lanIp":   services.PiholeLanIp(),
		"records": records,
	}})
}

func AddPiholeDns(c echo.Context) error {
	var data struct {
		Hostname string `json:"hostname"`
	}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = services.AddDns(data.Hostname)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Record created"})
}

func RemovePiholeDns(c echo.Context) error {
	err := services.RemoveDns(c.Param("hostname"), false)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Record removed"})
}

func GetJellyfinStreams(c echo.Context) error {
	streams, err := services.JellyfinStreams()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d streams", len(streams)), Data: streams})
}

func GetMediaPipeline(c echo.Context) error {
	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: services.GetPipeline()})
}

func GetQbitTorrents(c echo.Context) error {
	torrents, err := services.QbitTorrents()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: torrents})
}

var torrentHash = regexp.MustCompile(`^[a-fA-F0-9]{40,64}$`)

func QbitToggleTorrent(c echo.Context) error {
	hash := c.Param("hash")
	action := c.Param("action")
	if !torrentHash.MatchString(hash) || (action != "start" && action != "stop" && action != "remove") {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Both hash and action (\"start\", \"stop\", \"remove\") must be sent"})
	}

	err := services.QbitAction(hash, action)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}
