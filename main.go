package main

import (
	"front_desk/controller"
	"front_desk/models"
	"html/template"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func newTemplate() *models.Templates {
	return &models.Templates{
		Templates: template.Must(template.ParseGlob("view/*.html")),
	}
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())

		e.Static("/dist", "dist")
	e.Renderer = newTemplate()

	e.GET("/", controller.Root)
	e.GET("/paychecker", controller.PayChecker)

	e.Logger.Fatal(e.Start(":8080"))
}
