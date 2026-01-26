package features

import (
	"github.com/glgaspar/front_desk/features/login"
)

func CreateDatabase() error {
	return login.CreateDatabase()
}

func CheckForUsers() error {
	return new(login.LoginUser).CheckForUsers()
}

