package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/glgaspar/front_desk/features/login"
	"github.com/labstack/echo/v4"
)

func TestActionLabel(t *testing.T) {
	if got := actionLabel("POST", "/apps/create"); got != "created app" {
		t.Errorf("got %q", got)
	}
	if got := actionLabel("PATCH", "/whatever"); got != "PATCH /whatever" {
		t.Errorf("unknown routes should fall back to the route, got %q", got)
	}
}

func TestPathTarget(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("PUT", "/apps/toggleOnOff/abc/start?x=1", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetPath("/apps/toggleOnOff/:id/:toggle")
	c.SetParamNames("id", "toggle")
	c.SetParamValues("abc", "start")

	if got := pathTarget(c); got != "id=abc toggle=start x=1" {
		t.Errorf("got %q", got)
	}
}

func TestAdminOnly(t *testing.T) {
	t.Setenv("ENVIROMENT", "PROD")
	e := echo.New()
	handler := AdminOnly(func(c echo.Context) error { return c.String(200, "ok") })

	cases := []struct {
		user *login.SessionUser
		want int
	}{
		{nil, 403},
		{&login.SessionUser{Id: 2, IsAdmin: false}, 403},
		{&login.SessionUser{Id: 1, IsAdmin: true}, 200},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest("POST", "/apps/create", nil), rec)
		if tc.user != nil {
			c.Set("user", tc.user)
		}
		handler(c)
		if rec.Code != tc.want {
			t.Errorf("user %+v got %d want %d", tc.user, rec.Code, tc.want)
		}
	}
}
