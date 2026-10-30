package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/glgaspar/front_desk/features/activity"
	"github.com/labstack/echo/v4"
)

// what shows up on the activity page instead of the raw route
var actionLabels = map[string]string{
	"POST /register":                         "created user",
	"GET /login/logout":                      "logged out",
	"PUT /users/:id":                         "changed user",
	"PUT /users/:id/admin":                   "changed admin",
	"DELETE /users/:id":                      "deleted user",
	"PUT /apps/toggleOnOff/:id/:toggle":      "started/stopped app",
	"GET /apps/compose/:id":                  "opened compose",
	"POST /apps/compose/:id":                 "saved compose",
	"POST /apps/create":                      "created app",
	"DELETE /apps/remove/:id":                "deleted app",
	"GET /apps/logs/:id":                     "watched app logs",
	"PUT /apps/restart/:id":                  "restarted app",
	"POST /apps/update/:id":                  "updated app",
	"GET /apps/updates":                      "checked for updates",
	"POST /widgets":                          "registered widget",
	"PUT /widgets/toggle/:id/:toggle":        "changed widget",
	"PUT /widgets/order":                     "reordered widgets",
	"POST /cloudflare/config":                "changed cloudflare config",
	"POST /cloudflare/hostname":              "added cloudflare hostname",
	"DELETE /cloudflare/hostname/:hostname":  "removed cloudflare hostname",
	"POST /npm/config":                       "changed nginx proxy manager config",
	"POST /npm/hosts":                        "added proxy host",
	"PUT /npm/hosts/:id/certificate":         "changed proxy host certificate",
	"DELETE /npm/hosts/:id":                  "removed proxy host",
	"POST /services/:name/config":            "changed integration config",
	"POST /pihole/dns":                       "added pihole dns record",
	"DELETE /pihole/dns/:hostname":           "removed pihole dns record",
	"POST /qbittorrent/toggle/:hash/:action": "changed torrent",
	"POST /apps/explain/:id":                 "asked ollama about app",
	"POST /seerr/requests/:id/:action":       "answered seerr request",
	"POST /widgets/:name/config":             "changed widget config",
	"POST /widgets/:name/call":               "widget action",
	"POST /transmission/config":              "changed transmission config",
	"POST /transmission/toggle/:id/:action":  "changed torrent",
	"POST /notifications":                    "added notification channel",
	"PUT /notifications/:id":                 "changed notification channel",
	"DELETE /notifications/:id":              "removed notification channel",
	"POST /notifications/:id/test":           "tested notification channel",
}

// GETs are mostly the front polling stuff, only these are worth writing down
var loggedGets = map[string]bool{
	"/login/logout":     true,
	"/apps/compose/:id": true,
	"/apps/logs/:id":    true,
}

func actionLabel(method string, path string) string {
	if label, ok := actionLabels[method+" "+path]; ok {
		return label
	}
	return method + " " + path
}

func newEntry(c echo.Context, action string, target string) activity.Entry {
	entry := activity.Entry{Action: action, Target: target, Ip: c.RealIP()}
	if user := CurrentUser(c); user != nil {
		entry.UserId = &user.Id
		entry.UserName = user.UserName
	}
	return entry
}

func pathTarget(c echo.Context) string {
	var parts []string
	for i, name := range c.ParamNames() {
		values := c.ParamValues()
		if i < len(values) {
			parts = append(parts, name+"="+values[i])
		}
	}
	if c.Request().URL.RawQuery != "" {
		parts = append(parts, c.Request().URL.RawQuery)
	}
	return strings.Join(parts, " ")
}

// runs after the login check so it knows who did it. never looks at the body, so passwords stay out
func ActivityLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		method := c.Request().Method
		path := c.Path()

		if method == http.MethodOptions || path == "/login" || path == "/activity" {
			return next(c)
		}

		if method == http.MethodGet {
			if !loggedGets[path] && !(path == "/apps/updates" && c.QueryParam("force") == "true") {
				return next(c)
			}
			// written before running, logs is a stream that stays open for hours and logout forgets who you are
			activity.Log(newEntry(c, actionLabel(method, path), pathTarget(c)))
			return next(c)
		}

		entry := newEntry(c, actionLabel(method, path), pathTarget(c))
		err := next(c)

		// widgets polling through the api would bury everything else
		if skip, _ := c.Get("skipActivity").(bool); skip {
			return err
		}

		status := c.Response().Status
		if he, ok := err.(*echo.HTTPError); ok {
			status = he.Code
		}
		entry.Status = &status
		activity.Log(entry)
		return err
	}
}

func LogLogin(c echo.Context, userId int, userName string, ok bool) {
	entry := activity.Entry{UserName: userName, Action: "failed login", Ip: c.RealIP()}
	if ok {
		entry.UserId = &userId
		entry.Action = "logged in"
	}
	activity.Log(entry)
}

func GetActivity(c echo.Context) error {
	user := CurrentUser(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, Response{Status: false, Message: "You're not logged in"})
	}

	filter := activity.Filter{Action: c.QueryParam("action")}
	filter.Before, _ = strconv.ParseInt(c.QueryParam("before"), 10, 64)
	filter.Limit, _ = strconv.Atoi(c.QueryParam("limit"))

	if userParam := c.QueryParam("user"); userParam != "" {
		id, err := strconv.Atoi(userParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "user must be a valid integer"})
		}
		filter.UserId = &id
	}
	// regular users only see what they did
	if !user.IsAdmin {
		filter.UserId = &user.Id
	}

	list, err := activity.List(filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: list})
}

// for the stuff that only happens on the front, like opening a page
func TrackActivity(c echo.Context) error {
	var data struct {
		Action string `json:"action"`
		Target string `json:"target"`
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 4096))
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil || strings.TrimSpace(data.Action) == "" {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "action must be sent"})
	}

	activity.Log(newEntry(c, strings.TrimSpace(data.Action), strings.TrimSpace(data.Target)))
	return c.JSON(http.StatusOK, Response{Status: true, Message: "ok"})
}
