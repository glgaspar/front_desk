package main

import (
	"io"
	"log"
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
		var bypass bool = false
		if c.Path() == "/signup" { bypass = true }
		if bypass { return next(c) }
		if os.Getenv("FIRST_ACCESS") == "YES" {
			return c.Redirect(301, "/signup")
		}

		if c.Path() == "/login" { bypass = true }
		if bypass { return next(c) }

		cookie, err := c.Cookie("front_desk_awesome_cookie") 
		if (err != nil || cookie == nil) {
			return c.Redirect(301, "/login")
		}

		valid, err := controller.LoginValidator(cookie)
		if (err != nil || !valid) {
			return c.Redirect(301, "/login")
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

	e.GET("/", router.Root)
	e.GET("/login", router.Login)
	e.POST("/login", controller.Login)
	e.GET("/signup", router.Signup)
	e.POST("/signup", controller.Signup)
	e.GET("/paychecker", router.ShowPayChecker)
	e.PUT("/paychecker/flipTrack/:billId", controller.FlipPayChecker)
	e.POST("/paychecker/new", controller.NewPayChecker)

	e.Static("/static", "static")

	e.Logger.Fatal(e.Start(":8080"))
}
