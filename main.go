package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"text/template"

	"github.com/glgaspar/front_desk/router"

	"github.com/glgaspar/front_desk/controller"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)


type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func NewTemplates() *Template {
	tmpl := template.Must(template.ParseGlob("view/layouts/*.html"))
	template.Must(tmpl.ParseGlob("view/components/*.html"))
	template.Must(tmpl.ParseGlob("view/pages/*/*.html"))
	return &Template{
		templates: tmpl,		
	}
}


func redirect(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Path() == "/static*" { return next(c) } //css is free
				
		if os.Getenv("FIRST_ACCESS") == "YES" { //forcing you to create the first user
			return c.Redirect(http.StatusTemporaryRedirect, "/signup")
		}

		//making suer you are loged in
		cookie, err := c.Cookie("front_desk_awesome_cookie") 
		if (err != nil || cookie == nil) {
			if c.Path() == "/login" { return next(c) }
			return c.Redirect(http.StatusTemporaryRedirect, "/login")
		}
		valid, err := controller.LoginValidator(cookie)
		if (err != nil || !valid) {
			if c.Path() == "/login" { return next(c) }
			return c.Redirect(301, "/login")
		}

		//no need to come back here
		if c.Path() == "/login" || c.Path() == "/signup" {
			return c.Redirect(http.StatusTemporaryRedirect, "/home") 
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
    e.Renderer = NewTemplates()
    e.Use(middleware.Logger())
	e.Use(redirect)

	e.GET("/home", router.Root)

	e.GET("/login", router.Login)
	e.POST("/login", controller.Login)
	
	e.GET("/signup", router.Signup)
	e.POST("/signup", controller.Signup)
	
	e.GET("/paychecker", router.ShowPayChecker)
	e.PUT("/paychecker/flipTrack/:billId", controller.FlipPayChecker)
	e.POST("/paychecker/new", controller.NewPayChecker)

	e.GET("/timetracker", router.ShowTimeTracker)
	e.POST("/timetracker", controller.AddTimeTracker)

	e.Static("/static", "static")

	e.Logger.Fatal(e.Start(":8080"))
}
