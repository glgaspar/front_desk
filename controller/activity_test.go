package controller

import (
	"net/http/httptest"
	"testing"

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
