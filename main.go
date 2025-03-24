package main

import (
	"io"
	"log"
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

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("error reading .env %s", err.Error())
	}

	e := echo.New()
    e.Renderer = NewTemplates()
    e.Use(middleware.Logger())

	e.GET("/", router.Root)
	e.GET("/paychecker", router.ShowPayChecker)
	e.PUT("/paychecker/flipTrack/:billId", controller.FlipPayChecker)
	e.POST("/paychecker/new", controller.NewPayChecker)

	e.Static("/static", "static")

	e.Logger.Fatal(e.Start(":8080"))
}
