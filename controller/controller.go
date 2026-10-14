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
	"github.com/glgaspar/front_desk/features/integrations/npm"
	"github.com/glgaspar/front_desk/features/integrations/pihole"
	"github.com/glgaspar/front_desk/features/integrations/transmission"
	"github.com/glgaspar/front_desk/features/integrations/widgets"
	"github.com/glgaspar/front_desk/features/login"
	"github.com/glgaspar/front_desk/features/monitoring"
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
	warning, err := container.RemoveContainer()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	message := "Operation successful"
	if warning != "" {
		message += ". " + warning
	}
	return c.JSON(http.StatusOK, Response{Status: true, Message: message})
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

	if !enabled {
		return c.JSON(http.StatusOK, Response{Status: false, Message: "Cloudflare not configured"})
	}

	config, err := cloudflare.PublicConfig()
	if err != nil {
		return c.JSON(http.StatusOK, Response{Status: enabled, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: enabled, Message: "Operation successful", Data: config})
}

type CloudflareRoute struct {
	Hostname  string  `json:"hostname"`
	Service   string  `json:"service"`
	AppId     *string `json:"appId"`
	AppName   *string `json:"appName"`
	Managed   bool    `json:"managed"`
	DnsRecord bool    `json:"dnsRecord"`
}

func GetCloudflareRoutes(c echo.Context) error {
	ingress, err := cloudflare.ListIngress()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	records, err := cloudflare.ListDNSRecords()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	hasRecord := map[string]bool{}
	for _, r := range records {
		hasRecord[r.Name] = true
	}

	saved, err := cloudflare.ListSavedRoutes()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	savedDir := map[string]string{}
	for _, r := range saved {
		savedDir[r.Hostname] = r.AppDir
	}

	appList, err := new(apps.App).GetList()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	routes := []CloudflareRoute{}
	for _, rule := range ingress {
		if rule.Hostname == nil {
			continue
		}
		dir, managed := savedDir[*rule.Hostname]
		route := CloudflareRoute{
			Hostname:  *rule.Hostname,
			Service:   rule.Service,
			Managed:   managed,
			DnsRecord: hasRecord[*rule.Hostname],
		}
		for _, app := range appList {
			if (dir != "" && app.Dir == dir) || cloudflare.HostFromUrl(app.Url) == *rule.Hostname {
				route.AppId = &app.Id
				route.AppName = &app.Name
				break
			}
		}
		routes = append(routes, route)
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d routes found", len(routes)), Data: routes})
}

func GetCloudflareDNS(c echo.Context) error {
	records, err := cloudflare.ListDNSRecords()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d records found", len(records)), Data: records})
}

func AddCloudflareHostname(c echo.Context) error {
	var data struct {
		Hostname string `json:"hostname"`
		Port     string `json:"port"`
		AppId    string `json:"appId"`
	}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	dir := ""
	if data.AppId != "" {
		if !apps.ValidID(data.AppId) {
			return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid app id"})
		}
		container := apps.Container{ID: data.AppId}
		app, err := container.GetApp()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
		}
		dir = app.Dir
		if data.Port == "" {
			data.Port = app.ForwardPort()
		}
	}

	if _, err := strconv.Atoi(data.Port); err != nil || data.Hostname == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Hostname and a valid port are required"})
	}

	err = cloudflare.AddHostname(strings.ToLower(strings.TrimSpace(data.Hostname)), data.Port, dir)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Hostname created"})
}

func RemoveCloudflareHostname(c echo.Context) error {
	hostname := c.Param("hostname")
	if hostname == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Hostname must be sent"})
	}

	err := cloudflare.RemoveHostname(hostname, c.QueryParam("dns") == "true")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Hostname removed"})
}

func SetNPM(c echo.Context) error {
	var data npm.Config
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = data.SetNPM()
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	// trying to log in right away, better to find out the password is wrong now
	_, err = npm.ListProxyHosts()
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Saved, but " + err.Error()})
	}

	err = integrations.SetAvailable("nginxproxymanager")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetNPM(c echo.Context) error {
	enabled, err := integrations.CheckFor("nginxproxymanager")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	config, err := npm.PublicConfig()
	if err != nil {
		return c.JSON(http.StatusOK, Response{Status: false, Message: "Nginx Proxy Manager not configured"})
	}

	return c.JSON(http.StatusOK, Response{Status: enabled, Message: "Operation successful", Data: config})
}

type NPMHost struct {
	npm.ProxyHost
	AppId   *string `json:"appId"`
	AppName *string `json:"appName"`
	Managed bool    `json:"managed"`
}

func GetNPMHosts(c echo.Context) error {
	hosts, err := npm.ListProxyHosts()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	saved, err := npm.ListSavedRoutes()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	savedDir := map[int]string{}
	for _, r := range saved {
		savedDir[r.ProxyHostId] = r.AppDir
	}

	appList, err := new(apps.App).GetList()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	list := []NPMHost{}
	for _, host := range hosts {
		dir, managed := savedDir[host.Id]
		item := NPMHost{ProxyHost: host, Managed: managed}
		for _, app := range appList {
			appHost := cloudflare.HostFromUrl(app.Url)
			matches := dir != "" && app.Dir == dir
			for _, domain := range host.DomainNames {
				if appHost != "" && strings.EqualFold(domain, appHost) {
					matches = true
				}
			}
			if matches {
				item.AppId = &app.Id
				item.AppName = &app.Name
				break
			}
		}
		list = append(list, item)
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d hosts found", len(list)), Data: list})
}

func GetNPMCertificates(c echo.Context) error {
	certs, err := npm.ListCertificates()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d certificates found", len(certs)), Data: certs})
}

func AddNPMHost(c echo.Context) error {
	var data struct {
		Hostname      string `json:"hostname"`
		Port          string `json:"port"`
		AppId         string `json:"appId"`
		CertificateId *int   `json:"certificateId"`
	}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	dir := ""
	if data.AppId != "" {
		if !apps.ValidID(data.AppId) {
			return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Invalid app id"})
		}
		container := apps.Container{ID: data.AppId}
		app, err := container.GetApp()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
		}
		dir = app.Dir
		if data.Port == "" {
			data.Port = app.ForwardPort()
		}
	}

	certId := npm.DefaultCertificate()
	if data.CertificateId != nil {
		certId = *data.CertificateId
	}

	host, err := npm.AddProxyHost(data.Hostname, data.Port, dir, certId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Proxy host created", Data: host})
}

func SetNPMCertificate(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be a valid integer"})
	}

	var data struct {
		CertificateId int `json:"certificateId"`
	}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = npm.SetCertificate(id, data.CertificateId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Certificate updated"})
}

func RemoveNPMHost(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be a valid integer"})
	}

	err = npm.RemoveProxyHost(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Proxy host removed"})
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

func GetUptime(c echo.Context) error {
	uptime, err := monitoring.GetUptime(c.QueryParam("range"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: uptime})
}

func GetUptimeHistory(c echo.Context) error {
	app := c.QueryParam("app")
	if app == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "app must be sent"})
	}

	history, err := monitoring.GetUptimeHistory(app, c.QueryParam("range"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: history})
}

func GetMetricsHistory(c echo.Context) error {
	metrics, err := monitoring.GetMetricsHistory(c.QueryParam("range"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: metrics})
}

func GetNotifications(c echo.Context) error {
	channels, err := monitoring.ListChannels()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	masked := []monitoring.Channel{}
	for _, ch := range channels {
		masked = append(masked, ch.Masked())
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d channels found", len(masked)), Data: masked})
}

func readChannel(c echo.Context) (*monitoring.Channel, error) {
	var data monitoring.Channel
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}
	defer c.Request().Body.Close()

	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

func CreateNotification(c echo.Context) error {
	data, err := readChannel(c)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = data.Create()
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Channel created", Data: data.Masked()})
}

func UpdateNotification(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be a valid integer"})
	}

	data, err := readChannel(c)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	data.Id = id

	err = data.Update()
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Channel updated", Data: data.Masked()})
}

func DeleteNotification(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be a valid integer"})
	}

	err = monitoring.DeleteChannel(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Channel removed"})
}

func TestNotification(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Id must be a valid integer"})
	}

	err = monitoring.TestChannel(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Sent, go check it"})
}
