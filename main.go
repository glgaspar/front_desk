package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/glgaspar/front_desk/controller"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)


func redirect(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {			
		//allows you to create the first user	
		if os.Getenv("FIRST_ACCESS") == "YES" { 
			if c.Path() == "/register" { return next(c) }
		}

		//making sure you are loged in
		cookie, err := c.Cookie("front_desk_awesome_cookie") 
		if (err != nil || cookie == nil) {
			if c.Path() == "/login" { return next(c) }
		}

		valid, err := controller.LoginValidator(cookie)
		if (err != nil || !valid) {
			if c.Path() == "/login" { return next(c) }
			return c.Redirect(301, "/login")
		}
		
		// update cookie to extend session if expiring in lass than 1 hour
		if cookie.Expires.Sub(cookie.Expires.Add(-1 * time.Hour)).Hours() < 1 {
			cookie.Expires = cookie.Expires.Add(24 * time.Hour)
			c.SetCookie(cookie)
		}

		return next(c)
	}
}


func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("error reading .env %s", err.Error())
	}

	if err := controller.CheckForUsers(); err != nil { // just setting stuff up
		panic(err)
	} 

	e := echo.New()
    e.Use(middleware.Logger())
	e.Use(redirect)

	e.GET("/validate", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok") //returns ok if the validation in middleware passed
	})
	e.POST("/login", controller.Login)
	
	e.POST("/signup", controller.Signup)
	
	e.PUT("/paychecker/flipTrack/:billId", controller.FlipPayChecker)
	e.POST("/paychecker/new", controller.NewPayChecker)

	e.POST("/timetracker", controller.AddTimeTracker)


	e.Logger.Fatal(e.Start(":8080"))
}
