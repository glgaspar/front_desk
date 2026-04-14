package controller

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glgaspar/front_desk/features/apps"
	"github.com/glgaspar/front_desk/features/integrations"
	"github.com/glgaspar/front_desk/features/integrations/cloudflare"
	"github.com/glgaspar/front_desk/features/integrations/pihole"
	"github.com/glgaspar/front_desk/features/integrations/transmission"
	"github.com/glgaspar/front_desk/features/integrations/widgets"
	"github.com/glgaspar/front_desk/features/login"
	"github.com/glgaspar/front_desk/features/system"
	"github.com/glgaspar/front_desk/features/utils/messenger"
	"github.com/labstack/echo/v4"
)

type Response struct {
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var activeBuilds sync.Map

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

func GetWaitingBuilds(c echo.Context) error {
	var data []string
	activeBuilds.Range(func(key, value interface{}) bool {
		if appName, ok := key.(string); ok {
			data = append(data, appName)
		}
		return true
	})
	if data == nil {
		data = []string{}
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d pending builds found", len(data)), Data: data})
}

func ListenToBuild(c echo.Context) error {
	appName := c.Param("app")
	rawLogs := make(chan string)

	val, ok := activeBuilds.Load(appName)
	if !ok {
		c.Response().Write([]byte("data: [no active build]\n\n"))
		c.Response().Flush()
		return nil
	}
	buildStartTime := val.(int64)

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	ctx := c.Request().Context()

	go func() {
		defer close(rawLogs)
		err := messenger.Subscribe(ctx, "build-logs", rawLogs)
		if err != nil {
			c.Logger().Errorf("Kafka subscription for topic 'build-logs' failed: %v", err)
		}
	}()

	for {
		select {
		case line, ok := <-rawLogs:
			if !ok {
				c.Response().Write([]byte("data: [stream closed]\n\n"))
				c.Response().Flush()
				return nil
			}
			var logMsg apps.BuildLogMsg
			if err := json.Unmarshal([]byte(line), &logMsg); err == nil {
				if logMsg.App == appName && logMsg.Time >= buildStartTime {
					fmt.Fprintf(c.Response(), "data: %s\n\n", logMsg.Log)
					c.Response().Flush()
					if logMsg.Done {
						return nil
					}
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
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

	var dir string
	var appName string

	reader := strings.NewReader(data.Compose)

	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "front-desk.dir:") {
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid front-desk.dir label")
			}
			dir = strings.TrimSpace(parts[1])
			if dir[0] != '/' {
				dir = "/" + dir
			}
		}
		if strings.Contains(line, "front-desk.name:") {
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid front-desk.name label")
			}
			appName = strings.TrimSpace(parts[1])
		}
	}

	if dirInfo, err := os.Stat("/src/apps" + dir); err == nil && dirInfo.IsDir() {
		fileInfo, err := os.Stat("/src/apps" + dir + "/docker-compose.yml")
		if err == nil {
			if fileInfo.Size() > 1 {
				return c.JSON(http.StatusConflict, Response{Status: false, Message: "An app in this directory already exists."})
			}
		}
	}

	activeBuilds.Store(appName, time.Now().UnixNano())
	app := apps.App{}
	go func() {
		defer activeBuilds.Delete(appName)
		err = app.CreateApp(data, appName, dir)

		var finalMsg string
		if err != nil {
			finalMsg = err.Error()
		} else {
			finalMsg = "All operations finished."
		}

		b, _ := json.Marshal(map[string]interface{}{
			"app":  appName,
			"log":  finalMsg,
			"time": time.Now().UnixNano(),
			"done": true,
		})
		messenger.SendMessage("build-logs", string(b))

	}()

	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: struct {
		Topic string `json:"topic"`
	}{Topic: appName}})
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
	if action != "start" && action != "stop" && action != "remove" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Both Id (int64) and action (\"start\", \"stop\", \"remove\") must be sent"})
	}

	t := transmission.Transmission{}
	err = t.ToggleTorrent(id, action)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetWidgets(c echo.Context) error {
	homeOnly := c.QueryParam("homeOnly")
	if homeOnly != "true" && homeOnly != "false" && homeOnly != "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "homeOnly parameter must be \"true\" or \"false\" or empty"})
	}

	homeBool := homeOnly == "true"

	var widget = new(widgets.Widget)
	widgetList, err := widget.GetList(homeBool)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d Widgets found", len(widgetList)), Data: widgetList})
}

func CreateWidget(c echo.Context) error {
	var data widgets.Widget
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = data.CreateWidget()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Widget created successfully", Data: data})
}

func ToggleWidget(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be sent"})
	}
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be a valid integer"})
	}

	toggle := strings.ToLower(c.Param("toggle"))
	if toggle != "enabled" && toggle != "selected" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Toggle must be \"enabled\" or \"selected\""})
	}

	widget := widgets.Widget{Id: idInt}
	err = widget.Toggle(toggle)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: widget})
}
