package login

import (
	"errors"
	"hash/fnv"
	"log"
	"os"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/google/uuid"
)

type LoginUser struct {
	Id       int    `json:"id" db:"id"`
	UserName string `json:"userName" db:"userName"`
	Password string `json:"password" db:"password"`
}

type SessionCookie struct {
	Valid   bool
	Name    string
	Value   string
	Domain  string
	Expires time.Time
}

func (u *LoginUser) Login() (*SessionCookie, error) {
	session := new(SessionCookie)
	sessionToken := uuid.New()
	userCount := 0
	hashPass := fnv.New32a()

	_, err := hashPass.Write([]byte(u.Password))
	if err != nil {
		return nil, err
	}

	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	query := `
	select id
	from adm.users
	where
		username = ?
		and hash = ? `
	res, err := conn.Query(query, u.UserName, hashPass.Sum32())
	if err != nil {
		return nil, err
	}

	for res.Next() {
		res.Scan(&u.Id)
		userCount++
	}

	if userCount == 0 {
		session.Valid = false
		return session, nil
	}

	if userCount > 1 {
		return nil, errors.New("somehow you got multiple users with matching credentials")
	}

	sessionClearQuery := `
	delete from adm.activesessions
	where userid = ? 
	`
	sessionCreateQuery := `
	insert into adm.activesessions
	(userid, token)
	values (?,?)
	`

	tran, err := conn.Begin()
	if err != nil {
		return nil, err
	}

	_, err = tran.Exec(sessionClearQuery, u.Id)
	if err != nil {
		tran.Rollback()
		return nil, err
	}

	_, err = tran.Exec(sessionCreateQuery, u.Id, sessionToken)
	if err != nil {
		tran.Rollback()
		return nil, err
	}

	session.Valid = true
	session.Name = "front_desk_awesome_cookie"
	session.Value = sessionToken.String()
	session.Domain = os.Getenv("DOMAIN_NAME")
	session.Expires = time.Now().Add(time.Hour * 24)

	return session, nil
}

func (u *LoginUser) Create() (LoginUser, error) {
	hashPass := fnv.New32a()
	_, err := hashPass.Write([]byte(u.Password))
	if err != nil {
		return *u, err
	}

	conn, err := connection.Db()
	if err != nil {
		return *u, err
	}
	defer conn.Close()

	query := `
	insert into adm.users (username, password)
	values (?,?)
	RETURNING id, username`
	res, err := conn.Query(query, u.UserName, hashPass.Sum32())
	if err != nil {
		return *u, err
	}

	for res.Next() {
		res.Scan(&u.Id, &u.UserName)
	}

	if os.Getenv("FIRST_ACCESS") == "YES" {
		err := u.CheckForUsers()
		if err != nil {
			return *u, err
		}
	}

	return *u, nil
}

func (u *LoginUser) CheckForUsers() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	var count int
	query := `
	select count(id)
	from adm.users `
	res, err := conn.Query(query)
	if err != nil {
		return err
	}

	for res.Next() {
		res.Scan(&count)
	}

	if count < 1 {
		log.Println("no users created yet")
		err = os.Setenv("FIRST_ACCESS", "YES")
		if err != nil {
			return err
		}
		return nil
	}

	err = os.Setenv("FIRST_ACCESS", "NO")
	if err != nil {
		return err
	}
	return nil
}
