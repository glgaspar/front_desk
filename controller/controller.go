package controller

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/glgaspar/front_desk/features/components"
	"github.com/glgaspar/front_desk/features/login"
	"github.com/glgaspar/front_desk/features/paychecker"
	"github.com/glgaspar/front_desk/features/timetracker"
	"github.com/labstack/echo/v4"
)

func CheckForUsers() error {
	return new(login.LoginUser).CheckForUsers()
}

func Signup(c echo.Context) error {
	var data login.LoginUser
	var form components.FormData

	form.Data = data
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		form.Error = true
		form.Message = append(form.Message, err.Error())
		return c.Render(http.StatusUnprocessableEntity, "signupForm", form)
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		form.Error = true
		form.Message = append(form.Message, err.Error())
		return c.Render(http.StatusUnprocessableEntity, "signupForm", form)
	}

	newUser, err := data.Create()
	if err != nil {
		form.Error = true
		form.Message = append(form.Message, err.Error())
		return c.Render(http.StatusUnprocessableEntity, "signupForm", form)
	}
	form.Data = newUser
	return c.Render(http.StatusOK, "signupForm", form)
}

func Login(c echo.Context) error {
	var data login.LoginUser
	var form components.FormData
	var status int = http.StatusOK
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		status = http.StatusUnprocessableEntity
		form.Error = true
		form.Message = append(form.Message, err.Error())
		return err
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		status = http.StatusUnprocessableEntity
		form.Error = true
		form.Message = append(form.Message, err.Error())
	}
	form.Data = data

	newSession, err := data.Login()
	if err != nil {
		status = http.StatusUnprocessableEntity
		form.Error = true
		form.Message = append(form.Message, err.Error())
	}

	if form.Error {
		return c.Render(status, "loginForm", form)
	}

	cookie := http.Cookie{
		Name:    newSession.Name,
		Domain:  newSession.Domain,
		Value:   newSession.Value,
		Expires: newSession.Expires,
	}

	c.SetCookie(&cookie)
	return c.Render(status, "loginForm", form)
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

func FlipPayChecker(c echo.Context) error {
	log.Println("flipn track")
	var data = new(paychecker.Bill)
	id, err := strconv.Atoi(c.Param("billId"))
	if err != nil {
		c.Render(http.StatusUnprocessableEntity, "errorPopUp", err)
		return err
	}
	data.Id = id
	if err = data.FlipTrack(); err != nil {
		c.Render(http.StatusUnprocessableEntity, "errorPopUp", err)
		return err
	}

	c.Render(http.StatusOK, "paycheckerCard", data)
	return nil
}

func NewPayChecker(c echo.Context) error {
	var data paychecker.Bill
	var form components.FormData
	var status int = http.StatusOK
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		status = http.StatusUnprocessableEntity
		form.Error = true
		form.Message = append(form.Message, err.Error())
		return err
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		status = http.StatusUnprocessableEntity
		form.Error = true
		form.Message = append(form.Message, err.Error())
	}
	form.Data = data

	newBill, err := data.CreateBill()
	if err != nil {
		status = http.StatusUnprocessableEntity
		form.Error = true
		form.Message = append(form.Message, err.Error())
	}
	form.Data = newBill
	c.Render(status, "oob-paycheckerCard", form.Data)
	c.Render(status, "paycheckerAddNewModal", form)
	return nil
}

func AddTimeTracker(c echo.Context) error {
	log.Println("adding new time")
	var data = new(timetracker.Tracker)
	err := data.NewEntry()
	if err != nil {
		c.Render(http.StatusUnprocessableEntity, "errorPopUp", err)
		return err
	}

	return c.Render(http.StatusOK, "timeTrackerList", data)
}
