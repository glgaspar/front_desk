package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

	c.SetCookie(sessionCookie(newSession.Value, newSession.Expires))
	return c.JSON(http.StatusOK, Response{Status: true, Message: "Login successful"})
}

func LoginValidator(c *http.Cookie) (bool, time.Time, error) {
	cookie := c.Value
	if cookie == "" {
		return false, time.Time{}, nil
	}

	return login.LoginValidator(cookie)
}

// COOKIE_SECURE=true if you are behind https (you should be)
// COOKIE_SAMESITE=none if the front and the api are on different domains
func sessionCookie(value string, expires time.Time) *http.Cookie {
	cookie := &http.Cookie{
		Name:     "front_desk_awesome_cookie",
		Value:    value,
		Domain:   os.Getenv("DOMAIN_NAME"),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   os.Getenv("COOKIE_SECURE") == "true",
		SameSite: http.SameSiteLaxMode,
	}

	switch strings.ToLower(os.Getenv("COOKIE_SAMESITE")) {
	case "strict":
		cookie.SameSite = http.SameSiteStrictMode
	case "none":
		cookie.SameSite = http.SameSiteNoneMode
		cookie.Secure = true
	}

	return cookie
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

	cookie = sessionCookie("", time.Unix(0, 0))
	cookie.MaxAge = -1
	c.SetCookie(cookie)

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Logout successful"})
}

func RefreshSession(c *http.Cookie) (*http.Cookie, error) {
	expire, err := login.RefreshSession(c.Value)
	if err != nil {
		return nil, err
	}
	return sessionCookie(c.Value, expire), nil
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
						c.Response().Write([]byte("data: [stream closed]\n\n"))
						c.Response().Flush()
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
	if (toggle != "start" && toggle != "stop") || !apps.ValidID(id) {
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
	if !apps.ValidID(id) {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid id"})
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
	if !apps.ValidID(id) {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid id"})
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

	labels, err := apps.ParseComposeLabels(data.Compose)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	path, _ := apps.AppPath(labels.Dir)
	if dirInfo, err := os.Stat(path); err == nil && dirInfo.IsDir() {
		fileInfo, err := os.Stat(filepath.Join(path, "docker-compose.yml"))
		if err == nil {
			if fileInfo.Size() > 1 {
				return c.JSON(http.StatusConflict, Response{Status: false, Message: "An app in this directory already exists."})
			}
		}
	}

	tunnel := data.Tunnel != nil && *data.Tunnel
	started := startBuild(labels.Name, func() error {
		app := apps.App{}
		return app.CreateApp(data, labels.Name, labels.Dir, tunnel)
	})
	if !started {
		return c.JSON(http.StatusConflict, Response{Status: false, Message: "There is already a build running for " + labels.Name})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: struct {
		Topic string `json:"topic"`
	}{Topic: labels.Name}})
}

// runs the build in the background and tells whoever is listening when its done
func startBuild(appName string, build func() error) bool {
	_, running := activeBuilds.LoadOrStore(appName, time.Now().UnixNano())
	if running {
		return false
	}

	go func() {
		defer activeBuilds.Delete(appName)
		err := build()

		finalMsg := "All operations finished."
		if err != nil {
			finalMsg = err.Error()
		}

		apps.SendBuildDone(appName, finalMsg)
	}()

	return true
}

func RestartApp(c echo.Context) error {
	id := c.Param("id")
	if !apps.ValidID(id) {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid id"})
	}

	app := new(apps.App)
	err := app.Restart(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: app})
}

func UpdateApp(c echo.Context) error {
	id := c.Param("id")
	if !apps.ValidID(id) {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid id"})
	}

	container := apps.Container{ID: id}
	app, err := container.GetApp()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	started := startBuild(app.Name, app.Update)
	if !started {
		return c.JSON(http.StatusConflict, Response{Status: false, Message: "There is already a build running for " + app.Name})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: struct {
		Topic string `json:"topic"`
	}{Topic: app.Name}})
}

func CheckAppUpdate(c echo.Context) error {
	id := c.Param("id")
	if !apps.ValidID(id) {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid id"})
	}

	container := apps.Container{ID: id}
	app, err := container.GetApp()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	status, err := apps.CheckUpdate(app.ImageRef, c.QueryParam("force") == "true")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: status})
}

func GetAppUpdates(c echo.Context) error {
	updates, err := apps.GetAllUpdates(c.QueryParam("force") == "true")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d images checked", len(updates)), Data: updates})
}

func GetAppStats(c echo.Context) error {
	stats, err := apps.GetStats()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: stats})
}

func RemoveContainer(c echo.Context) error {
	id := c.Param("id")
	if !apps.ValidID(id) {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid id"})
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
	if !apps.ValidID(id) {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid id"})
	}

	app := apps.App{Id: id}
	err := app.GetApp()
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	err = app.GetLogs(c.Request().Context(), func(line string) {
		fmt.Fprintf(c.Response(), "data: %s\n\n", line)
		c.Response().Flush()
	})
	if err != nil {
		fmt.Fprintf(c.Response(), "data: %s\n\n", err.Error())
	}

	c.Response().Write([]byte("data: [connection closed]\n\n"))
	c.Response().Flush()
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
