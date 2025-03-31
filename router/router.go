package router

import (
	"log"
	"net/http"

	"github.com/glgaspar/front_desk/features/paychecker"
	"github.com/glgaspar/front_desk/features/root"
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

