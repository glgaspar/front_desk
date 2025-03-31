package controller

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/glgaspar/front_desk/features/components"
	"github.com/glgaspar/front_desk/features/paychecker"
	"github.com/glgaspar/front_desk/features/login"
	"github.com/labstack/echo/v4"
)

func CheckForUsers() error {
	return new(login.LoginUser).CheckForUsers()
}

func Signup(c echo.Context) error {
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

	newUser, err := data.Create()
	if err != nil {
		status = http.StatusUnprocessableEntity
		form.Error = true
		form.Message = append(form.Message, err.Error())	
	}
	form.Data = newUser
	c.Render(status, "newUserForm", form)
	return nil
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