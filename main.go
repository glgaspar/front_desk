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

		user, expire, err := controller.LoginValidator(cookie)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "Something went wrong: "+ err.Error()})
		}
		if user == nil {
			return c.JSON(http.StatusUnauthorized, controller.Response{Status: false, Message: "You're not logged in"})
		}
		c.Set("user", user)
		
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
	e.Use(controller.ActivityLogger)

	// controller.AdminOnly on everything that changes the state of something
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok") 
	})

	e.POST("/register", controller.Signup)
	e.POST("/login", controller.Login)
	e.GET("/login/logout", controller.Logout)
	e.GET("/me", controller.Me)
	e.GET("/activity", controller.GetActivity)
	e.POST("/activity", controller.TrackActivity)
	e.GET("/users", controller.GetUsers)
	e.PUT("/users/:id", controller.UpdateUser)
	e.PUT("/users/:id/admin", controller.SetUserAdmin, controller.AdminOnly)
	e.DELETE("/users/:id", controller.DeleteUser, controller.AdminOnly)
	
	e.GET("/validate", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok") //returns ok if the middleware validation passed
	})

	e.GET("/apps", controller.GetApps)
	e.GET("/apps/waitingBuilds", controller.GetWaitingBuilds)
	e.GET("/apps/waitingBuilds/:app", controller.ListenToBuild)
	e.PUT("/apps/toggleOnOff/:id/:toggle", controller.AppsToggleOnOFF, controller.AdminOnly)
	e.GET("/apps/compose/:id", controller.GetCompose, controller.AdminOnly)
	e.POST("/apps/compose/:id", controller.SaveCompose, controller.AdminOnly)
	e.POST("/apps/create", controller.CreateApp, controller.AdminOnly)
	e.DELETE("/apps/remove/:id", controller.RemoveContainer, controller.AdminOnly)
	e.GET("/apps/logs/:id", controller.GetLogs)
	e.PUT("/apps/restart/:id", controller.RestartApp, controller.AdminOnly)
	e.POST("/apps/update/:id", controller.UpdateApp, controller.AdminOnly)
	e.GET("/apps/update/:id", controller.CheckAppUpdate)
	e.GET("/apps/updates", controller.GetAppUpdates)
	e.GET("/apps/stats", controller.GetAppStats)

	e.GET("/system/usage", controller.GetSystemUsage)

	e.GET("/monitoring/uptime", controller.GetUptime)
	e.GET("/monitoring/uptime/history", controller.GetUptimeHistory)
	e.GET("/monitoring/metrics", controller.GetMetricsHistory)

	e.GET("/notifications", controller.GetNotifications)
	e.POST("/notifications", controller.CreateNotification, controller.AdminOnly)
	e.PUT("/notifications/:id", controller.UpdateNotification, controller.AdminOnly)
	e.DELETE("/notifications/:id", controller.DeleteNotification, controller.AdminOnly)
	e.POST("/notifications/:id/test", controller.TestNotification, controller.AdminOnly)
	
	e.POST("/widgets", controller.CreateWidget)
	e.GET("/widgets", controller.GetWidgets)
	e.PUT("/widgets/toggle/:id/:toggle", controller.ToggleWidget, controller.AdminOnly)

	e.POST("/cloudflare/config", controller.SetCloudflare, controller.AdminOnly)
	e.GET("/cloudflare/config", controller.GetCloudflare)
	e.GET("/cloudflare/routes", controller.GetCloudflareRoutes)
	e.GET("/cloudflare/dns", controller.GetCloudflareDNS)
	e.POST("/cloudflare/hostname", controller.AddCloudflareHostname, controller.AdminOnly)
	e.DELETE("/cloudflare/hostname/:hostname", controller.RemoveCloudflareHostname, controller.AdminOnly)

	e.POST("/npm/config", controller.SetNPM, controller.AdminOnly)
	e.GET("/npm/config", controller.GetNPM)
	e.GET("/npm/hosts", controller.GetNPMHosts)
	e.GET("/npm/certificates", controller.GetNPMCertificates)
	e.POST("/npm/hosts", controller.AddNPMHost, controller.AdminOnly)
	e.PUT("/npm/hosts/:id/certificate", controller.SetNPMCertificate, controller.AdminOnly)
	e.DELETE("/npm/hosts/:id", controller.RemoveNPMHost, controller.AdminOnly)

	// the integrations that are just an url and a key share the same config
	e.POST("/services/:name/config", controller.SetService, controller.AdminOnly)
	e.GET("/services/:name/config", controller.GetService)

	e.GET("/pihole/history", controller.PiholeHistory)
	e.GET("/pihole/dns", controller.GetPiholeDns)
	e.POST("/pihole/dns", controller.AddPiholeDns, controller.AdminOnly)
	e.DELETE("/pihole/dns/:hostname", controller.RemovePiholeDns, controller.AdminOnly)





	e.POST("/transmission/config", controller.SetTransmission, controller.AdminOnly)
	e.GET("/transmission/config", controller.GetTransmission)
	e.POST("/transmission/toggle/:id/:action", controller.TransmissionToggleTorrent, controller.AdminOnly)
	e.GET("/transmission/torrents", controller.GetTransmissionTorrents)



	e.Logger.Fatal(e.Start(":8080"))
}
