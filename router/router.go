package router

import (
	"log"
	"net/http"

	"github.com/glgaspar/front_desk/features/paychecker"
	"github.com/glgaspar/front_desk/features/root"
	"github.com/glgaspar/front_desk/features/timetracker"
	"github.com/labstack/echo/v4"
)

func Signup(c echo.Context) error {
	err := c.Render(http.StatusOK, "signup", nil)
	if err != nil {
		c.Render(http.StatusTeapot, "error", err)
		return err
	}
	return nil
}

func Login(c echo.Context) error {
	err := c.Render(http.StatusOK, "login", nil)
	if err != nil {
		c.Render(http.StatusTeapot, "error", err)
		return err
	}
	return nil
}

func Root(c echo.Context) error {
	log.Println("Fetching apps")
	var data = root.RootConfig{}
	if err := data.Generate(); err != nil {
		c.Render(http.StatusUnprocessableEntity, "error", err)
		return err
	}
	err := c.Render(http.StatusOK, "rootdata", data)
	if err != nil {
		c.Render(http.StatusUnprocessableEntity, "error", err)
		return err
	}
	return nil
}

func ShowPayChecker(c echo.Context) error {
	log.Println("Fetching paychecker bills")
	data, err := new(paychecker.Bill).GetAllBills()
	if err != nil {
		c.Render(http.StatusUnprocessableEntity, "error", err)
		return err
	}

	err = c.Render(http.StatusOK, "paychecker", data)
	if err != nil {
		c.Render(http.StatusUnprocessableEntity, "error", err)
		return err
	}
	return nil
}

func ShowTimeTracker(c echo.Context) error {
	log.Println("Fetching timetracker timesheet")
	data := new(timetracker.Tracker)
	err := data.GetTodayList()
	if err != nil {
		c.Render(http.StatusUnprocessableEntity, "error", err)
		return err
	}

	err = c.Render(http.StatusOK, "timeTracker", data)
	if err != nil {
		c.Render(http.StatusUnprocessableEntity, "error", err)
		return err
	}
	return err
}