package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/glgaspar/front_desk/features/apps"
	"github.com/glgaspar/front_desk/features/integrations"
	"github.com/glgaspar/front_desk/features/integrations/cloudflare"
	"github.com/glgaspar/front_desk/features/integrations/pihole"
	"github.com/glgaspar/front_desk/features/integrations/transmission"
	"github.com/glgaspar/front_desk/features/login"
	"github.com/glgaspar/front_desk/features/system"
	"github.com/labstack/echo/v4"
)

type Response struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Signup(c echo.Context) error {
	var data login.LoginUser
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	_, err = data.Create()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "User created successfully"})
}

func Login(c echo.Context) error {
	var data login.LoginUser
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	newSession, err := data.Login()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	if !newSession.Valid {
		return c.JSON(http.StatusUnauthorized, Response{Status: false, Message: "Login failed. Check your credentials"})
	}

	cookie := http.Cookie{
		Name:    newSession.Name,
		Domain:  newSession.Domain,
		Value:   newSession.Value,
		Expires: newSession.Expires,
	}

	c.SetCookie(&cookie)
	return c.JSON(http.StatusOK, Response{Status: true, Message: "Login successful"})
}

func LoginValidator(c *http.Cookie) (bool, error) {
	cookie := c.Value
	if cookie == "" {
		return false, nil
	}

	valid, err := login.LoginValidator(cookie)
	if err != nil {
		return false, err
	}
	return valid, nil
}

func Logout(c echo.Context) error {
	var user login.LoginUser
	cookie, err := c.Cookie("front_desk_awesome_cookie")
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = user.Logout(cookie)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	cookie.Value = ""
	cookie.MaxAge = -1
	cookie.Path = "/"
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Logout successful"})
}
func RefreshCookie(c *http.Cookie) (*http.Cookie, error) {
	valid, err := login.RefreshCookie(c)
	if err != nil {
		return c, err
	}
	return valid, nil
}

func GetApps(c echo.Context) error {
	var app = new(apps.App)
	appList, err := app.GetList()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d Apps found", len(appList)), Data: appList})
}

func AppsToggleOnOFF(c echo.Context) error {
	id := c.Param("id")
	toggle := c.Param("toggle")
	if (toggle != "start" && toggle != "stop") || id == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Both Id (str) and toggle (\"start\", \"stop\") must be sent"})
	}

	var app = new(apps.App)
	err := app.ToggleOnOFF(id, toggle)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: app})
}

func GetSystemUsage(c echo.Context) error {
	var data = new(system.SystemUsage)
	err := data.GetSystemUsage()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: data})
}

func GetCompose(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be sent"})
	}

	container := apps.Container{ID: id}
	data, err := container.GetCompose()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: data})
}

func SaveCompose(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be sent"})
	}
	var data apps.Compose
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	container := apps.Container{ID: id}
	newApp, err := container.SaveCompose(data)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: newApp})
}

func CreateApp(c echo.Context) error {
	var data apps.Compose
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	app := apps.App{}
	err = app.CreateApp(data)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: app})
}

func RemoveContainer(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be sent"})
	}

	container := apps.Container{ID: id}
	err := container.RemoveContainer()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetLogs(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: "Id must be sent"})
	}

	logs := make(chan string)
	app := apps.App{Id: id}

	err := app.GetApp()
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	go func(c *echo.Context, logs *chan string) {
		for {
			select {
			case line, ok := <-*logs:
				if !ok {
					(*c).Response().Write([]byte("data: [connection closed]\n\n"))
					(*c).Response().Flush()
					return
				}
				fmt.Fprintf((*c).Response(), "data: %s\n\n", line)
				(*c).Response().Flush()
			case <-(*c).Request().Context().Done():
				return
			}
		}
	}(&c, &logs)

	err = app.GetLogs(&logs)
	if err != nil {
		c.Response().Write([]byte(err.Error()))
		c.Response().Flush()
		return err
	}

	return nil
}

func SetCloudflare(c echo.Context) error {
	var data cloudflare.Config
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = data.SetCloudflare()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	err = integrations.SetAvailable("cloudflare")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetCloudflare(c echo.Context) error {
	enabled, err := integrations.CheckFor("cloudflare")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: enabled, Message: "Operation successful"})
}

func SetPihole(c echo.Context) error {
	var data pihole.Config
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = data.SetPihole()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	err = integrations.SetAvailable("pihole")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetPihole(c echo.Context) error {
	enabled, err := integrations.CheckFor("pihole")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: enabled, Message: "Operation successful"})
}

func PiholeHistory(c echo.Context) error {
	pihole := pihole.Pihole{}
	history, err := pihole.GetHistory()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: history})
}

func SetTransmission(c echo.Context) error {
	t := transmission.Config{}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &t); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = t.SetTransmission()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	err = integrations.SetAvailable("transmission")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetTransmission(c echo.Context) error {
	enabled, err := integrations.CheckFor("transmission")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: enabled, Message: "Operation successful"})
}

func GetTransmissionTorrents(c echo.Context) error {
	t := transmission.Transmission{}
	torrents, err := t.GetAllTorrents()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	
	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: torrents})
}

func TransmissionToggleTorrent(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be a valid integer"})
	}

	action := c.Param("action")
	if action != "start" && action != "stop" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Both Id (int64) and action (\"start\", \"stop\") must be sent"})
	}

	t := transmission.Transmission{}
	err = t.ToggleTorrent(id, action)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}