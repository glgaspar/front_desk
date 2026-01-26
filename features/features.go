package features

import (
	"log"

	"github.com/glgaspar/front_desk/features/integrations/cloudflare"
	"github.com/glgaspar/front_desk/features/integrations/pihole"
	"github.com/glgaspar/front_desk/features/login"
)

func CreateDatabase() error {
	return login.CreateDatabase()
}

func CheckForUsers() error {
	return new(login.LoginUser).CheckForUsers()
}

func CheckForPihole() error {
	var pihole = pihole.Pihole{}
	err := pihole.CheckForPihole()
	if err != nil {
		return err
	}
	if pihole.Enabled {
		log.Println("pihole available")
	} else {
		log.Println("pihole not available")
	}
	return nil
}

func CheckForCloudflare() error {
	data := new(cloudflare.Config)
	data.CheckForCloudflare()
	if data.Enabled {
		log.Println("cloudflare available")
	} else {
		log.Println("cloudflare not available")
	}
	return nil
}
