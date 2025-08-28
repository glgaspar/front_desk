package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/glgaspar/front_desk/features/apps"
	"github.com/glgaspar/front_desk/features/features"
	"github.com/glgaspar/front_desk/features/login"
	"github.com/glgaspar/front_desk/features/paychecker"
	"github.com/glgaspar/front_desk/features/timetracker"
	"github.com/labstack/echo/v4"
)

type Response struct {
	Status 	bool	  	`json:"status"`
	Message string 		`json:"message"`
	Data 	interface{} `json:"data"`
}

func CheckForUsers() error {
	return new(login.LoginUser).CheckForUsers()
}

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

func GetAllBills(c echo.Context) error {
	log.Println("Fetching paychecker bills")
	data, err := new(paychecker.Bill).GetAllBills()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	
	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d bills found", len(data)), Data: data})
}

func FlipPayChecker(c echo.Context) error {
	log.Println("flipn track")
	var data = new(paychecker.Bill)
	id, err := strconv.Atoi(c.Param("billId"))
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	data.Id = id
	if err = data.FlipTrack(); err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: data})
}

func NewPayChecker(c echo.Context) error {
	var data paychecker.Bill
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	defer c.Request().Body.Close()

	if err := json.Unmarshal(body, &data); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}

	err = data.CreateBill()
	if err != nil {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: data})
	
}

func ShowTimeTracker(c echo.Context) error {
	log.Println("Fetching timetracker timesheet")
	data := new(timetracker.Tracker)
	err := data.GetTodayList()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	
	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d timestamps found", len(data.List)), Data: data.List})
}

func AddTimeTracker(c echo.Context) error {
	log.Println("adding new time")
	var data = new(timetracker.Tracker)
	err := data.NewEntry()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}
	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: data})
}

func GetApps(c echo.Context) error {
	var app = new(apps.App)
	appList, err := app.GetList()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d Apps found", len(*appList)), Data: appList})
}

func SetLink(c echo.Context) error {
	var app = new(apps.App)
	id := c.Param("id")
	link := c.Param("link")

	if id == "" || link == "" {
		return c.JSON(http.StatusUnprocessableEntity, Response{Status: false, Message: "Id and link must be provided"})
	}

	app.ID = id

	app.SetLink(link)
	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: app})
}

func GetFeatures(c echo.Context) error {
	var feature = new(features.Feature)
	featureList := feature.Show()
	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d features found", len(featureList)), Data: featureList})
}