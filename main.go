package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/glgaspar/front_desk/controller"
	"github.com/glgaspar/front_desk/features"
	"github.com/glgaspar/front_desk/features/integrations"
	"github.com/glgaspar/front_desk/features/monitoring"
	"github.com/glgaspar/front_desk/features/utils/messenger"
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

		valid, expire, err := controller.LoginValidator(cookie)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "Something went wrong: "+ err.Error()})
		}
		if !valid {
			return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "You're not logged in"})
		}
		
		// update cookie to extend session if expiring in lass than 1 hour
		// the browser never sends the expire date back so we check the one in the db
		if time.Until(expire) < time.Hour {
			newCookie, err := controller.RefreshSession(cookie)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "Something went wrong: "+ err.Error()})
			}
			c.SetCookie(newCookie)
		}

		return next(c)
	}
}


func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("error reading .env %s", err.Error())
	}
	
	origins := os.Getenv("FRONT_END_URL")
	credentials := true
	enviroment := os.Getenv("ENVIROMENT")
	if enviroment == "DEV" {
		credentials = false
		origins = "*"	
	}
	
	log.Printf("Starting Front Desk in %s environment", enviroment)

	redBg := "\033[41m"
	greenBg := "\033[42m"
	reset := "\033[0m"

	log.Println("checking database tables... ")
	if err := features.CreateDatabase(); err != nil {
		log.Printf("%sFAILED%s: %v", redBg, reset, err)
		panic(err)
	}
	log.Printf("%sOK%s", greenBg, reset)

	log.Println("checking for users... ")
	if err := features.CheckForUsers(); err != nil {
		log.Printf("%sFAILED%s: %v", redBg, reset, err)
		panic(err)
	}
	log.Printf("%sOK%s", greenBg, reset)

	log.Println("checking integrations... ")
	if err := integrations.CheckAll(); err != nil {
		panic(err)
	}

	log.Println("ensuring build-logs topic exists... ")
	_ = messenger.CreateTopic("build-logs")

	log.Println("starting monitoring... ")
	monitoring.Start(context.Background())

	log.Printf("%sall checks done%s", greenBg, reset)
	log.Println("starting server")

	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{origins},
		AllowCredentials: credentials,
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

    e.Use(middleware.Logger())
	
	if enviroment != "DEV" {
		e.Use(authentication)
	}

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok") 
	})

	e.POST("/register", controller.Signup)
	e.POST("/login", controller.Login)
	e.GET("/login/logout", controller.Logout)
	
	e.GET("/validate", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok") //returns ok if the middleware validation passed
	})

	e.GET("/apps", controller.GetApps)
	e.GET("/apps/waitingBuilds", controller.GetWaitingBuilds)
	e.GET("/apps/waitingBuilds/:app", controller.ListenToBuild)
	e.PUT("/apps/toggleOnOff/:id/:toggle", controller.AppsToggleOnOFF)
	e.GET("/apps/compose/:id", controller.GetCompose)
	e.POST("/apps/compose/:id", controller.SaveCompose)
	e.POST("/apps/create", controller.CreateApp)
	e.DELETE("/apps/remove/:id", controller.RemoveContainer)
	e.GET("/apps/logs/:id", controller.GetLogs)
	e.PUT("/apps/restart/:id", controller.RestartApp)
	e.POST("/apps/update/:id", controller.UpdateApp)
	e.GET("/apps/update/:id", controller.CheckAppUpdate)
	e.GET("/apps/updates", controller.GetAppUpdates)
	e.GET("/apps/stats", controller.GetAppStats)

	e.GET("/system/usage", controller.GetSystemUsage)

	e.GET("/monitoring/uptime", controller.GetUptime)
	e.GET("/monitoring/uptime/history", controller.GetUptimeHistory)
	e.GET("/monitoring/metrics", controller.GetMetricsHistory)

	e.GET("/notifications", controller.GetNotifications)
	e.POST("/notifications", controller.CreateNotification)
	e.PUT("/notifications/:id", controller.UpdateNotification)
	e.DELETE("/notifications/:id", controller.DeleteNotification)
	e.POST("/notifications/:id/test", controller.TestNotification)
	
	e.POST("/widgets", controller.CreateWidget)
	e.GET("/widgets", controller.GetWidgets)
	e.PUT("/widgets/toggle/:id/:toggle", controller.ToggleWidget)

	e.POST("/cloudflare/config", controller.SetCloudflare)
	e.GET("/cloudflare/config", controller.GetCloudflare)
	e.GET("/cloudflare/routes", controller.GetCloudflareRoutes)
	e.GET("/cloudflare/dns", controller.GetCloudflareDNS)
	e.POST("/cloudflare/hostname", controller.AddCloudflareHostname)
	e.DELETE("/cloudflare/hostname/:hostname", controller.RemoveCloudflareHostname)

	e.POST("/pihole/config", controller.SetPihole)
	e.GET("/pihole/config", controller.GetPihole)
	e.GET("/pihole/history", controller.PiholeHistory)

	e.POST("/transmission/config", controller.SetTransmission)
	e.GET("/transmission/config", controller.GetTransmission)
	e.POST("/transmission/toggle/:id/:action", controller.TransmissionToggleTorrent)
	e.GET("/transmission/torrents", controller.GetTransmissionTorrents)



	e.Logger.Fatal(e.Start(":8080"))
}
