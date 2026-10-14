package login

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/glgaspar/front_desk/connection"
)

type User struct {
	Id       int    `json:"id"`
	UserName string `json:"userName"`
	IsAdmin  bool   `json:"isAdmin"`
}

// you can mess with yourself, admins can mess with everyone
func CanEditUser(current *SessionUser, targetId int) bool {
	if current == nil {
		return false
	}
	return current.IsAdmin || current.Id == targetId
}

// never leave the place without an admin
func CanLoseAdmin(target User, admins int) bool {
	return !target.IsAdmin || admins > 1
}

func ListUsers() ([]User, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(`select id, username, is_admin from frontdesk.users order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		err = rows.Scan(&u.Id, &u.UserName, &u.IsAdmin)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func GetUser(id int) (*User, error) {
	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	var u User
	err = conn.QueryRow(`select id, username, is_admin from frontdesk.users where id = $1`, id).Scan(&u.Id, &u.UserName, &u.IsAdmin)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func UsernameTaken(name string, exceptId int) (bool, error) {
	conn, err := connection.Db()
	if err != nil {
		return false, err
	}

	var taken bool
	err = conn.QueryRow(`select exists (select 1 from frontdesk.users where lower(username) = lower($1) and id <> $2)`, strings.TrimSpace(name), exceptId).Scan(&taken)
	return taken, err
}

func AdminCount() (int, error) {
	conn, err := connection.Db()
	if err != nil {
		return 0, err
	}

	var count int
	err = conn.QueryRow(`select count(*) from frontdesk.users where is_admin`).Scan(&count)
	return count, err
}

func CheckPassword(id int, password string) (bool, error) {
	conn, err := connection.Db()
	if err != nil {
		return false, err
	}

	var stored string
	err = conn.QueryRow(`select password from frontdesk.users where id = $1`, id).Scan(&stored)
	if err != nil {
		return false, err
	}
	ok, _ := verifyPassword(stored, password)
	return ok, nil
}

func Rename(id int, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 50 {
		return fmt.Errorf("name must have between 1 and 50 characters")
	}

	taken, err := UsernameTaken(name, id)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("%s is already taken", name)
	}

	conn, err := connection.Db()
	if err != nil {
		return err
	}
	_, err = conn.Exec(`update frontdesk.users set username = $1 where id = $2`, name, id)
	return err
}

// also kicks every other session of that user, keepToken is the one doing the change
func SetPassword(id int, password string, keepToken string) error {
	if len(password) < 4 {
		return fmt.Errorf("password is too short")
	}

	hash, err := hashPassword(password)
	if err != nil {
		return err
	}

	conn, err := connection.Db()
	if err != nil {
		return err
	}

	tran, err := conn.Begin()
	if err != nil {
		return err
	}

	_, err = tran.Exec(`update frontdesk.users set password = $1 where id = $2`, hash, id)
	if err != nil {
		tran.Rollback()
		return err
	}

	_, err = tran.Exec(`delete from frontdesk.activesessions where userid = $1 and token <> $2`, id, keepToken)
	if err != nil {
		tran.Rollback()
		return err
	}

	return tran.Commit()
}

func SetAdmin(id int, admin bool) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	_, err = conn.Exec(`update frontdesk.users set is_admin = $1 where id = $2`, admin, id)
	return err
}

func DeleteUser(id int) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	_, err = conn.Exec(`delete from frontdesk.users where id = $1`, id)
	return err
}
