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


func authentication(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		//healthcheck
		if c.Path() == "/" { return next(c) }
		
		//allows you to create the first user	
		if os.Getenv("FIRST_ACCESS") == "YES" { 
			if c.Path() == "/register" { return next(c) }
		}

		//you dont need to be loged in to log in
		if c.Path() == "/login" { return next(c) }
		
		//making sure you are loged in
		cookie, err := c.Cookie("front_desk_awesome_cookie") 
		if err != nil {
			return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "Something went wrong: "+ err.Error()})
		}
		if cookie == nil {
			return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "You're not logged in"})
		}

		valid, err := controller.LoginValidator(cookie)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "Something went wrong: "+ err.Error()})
		}
		if !valid {
			return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "You're not logged in"})
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
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{os.Getenv("FRONT_END_URL")},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))
    e.Use(middleware.Logger())
	e.Use(authentication)

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok") 
	})

	e.POST("/register", controller.Signup)
	e.POST("/login", controller.Login)
	e.GET("/validate", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok") //returns ok if the middleware validation passed
	})

	e.GET("/apps", controller.GetApps)
	e.PUT("/apps/:id/:link", controller.SetLink)

	e.GET("/features", controller.GetFeatures)

	// e.GET("/system", controller.GetSystem)
	
	e.GET("/paychecker", controller.GetAllBills)
	e.PUT("/paychecker/flipTrack/:billId", controller.FlipPayChecker)
	e.POST("/paychecker/new", controller.NewPayChecker)

	e.GET("/timetracker", controller.ShowTimeTracker)
	e.POST("/timetracker", controller.AddTimeTracker)


	e.Logger.Fatal(e.Start(":8080"))
}
