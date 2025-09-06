package login

import (
	"errors"
	"hash/fnv"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/google/uuid"
)

func LoginValidator(cookie string) (bool, error) {
	var valid bool
	conn, err := connection.Db()
	if err != nil {
		return valid, err
	}
	defer conn.Close()

	query := `
	select 
	coalesce(case
		when count(userId) = 0 then false
		else true 
	end,false) 
	from adm.activesessions
	where token = $1 and expire >= now() `
	res, err := conn.Query(query, cookie)
	if err != nil {
		return valid, err
	}

	for res.Next() {
		res.Scan(&valid)
	}

	return valid, nil
}

func RefreshCookie(cookie *http.Cookie) (*http.Cookie, error) {
	cookie.Expires = cookie.Expires.Add(24 * time.Hour)
	conn, err := connection.Db()
	if err != nil {
		return cookie, err
	}
	defer conn.Close()

	query := `
	update adm.activesessions
		token $1,
		expire = $2
	where
		token = $1
	RETURNING token, expire`
	
	res, err := conn.Query(query, (*cookie).Value, (*cookie).Expires)
	if err != nil {
		return cookie, err
	}

	for res.Next() {
		res.Scan(&(*cookie).Value, &(*cookie).Expires)
	}

	return cookie, nil
}

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
	session := SessionCookie{
		Name: "front_desk_awesome_cookie",
		Value: uuid.New().String(),
		Domain: os.Getenv("DOMAIN_NAME"),
		Expires: time.Now().Add(time.Hour * 24),
	}

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
		username = $1
		and password = $2 `
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
		return &session, nil
	}

	if userCount > 1 {
		return nil, errors.New("somehow you got multiple users with matching credentials")
	}

	sessionClearQuery := `
	delete from adm.activesessions
	where userid = $1 and expire < now()
	`
	sessionCreateQuery := `
	insert into adm.activesessions
	(userid, token, expire)
	values ($1,$2,$3)
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

	_, err = tran.Exec(sessionCreateQuery, u.Id, session.Value, session.Expires)
	if err != nil {
		tran.Rollback()
		return nil, err
	}

	err = tran.Commit()
	if err != nil {
		tran.Rollback()
		return nil, err
	}

	session.Valid = true

	return &session, nil
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
	values ($1,$2)
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

	log.Println("we got users")
	err = os.Setenv("FIRST_ACCESS", "NO")
	if err != nil {
		return err
	}
	return nil
}

func (u *LoginUser) Logout(cookie *http.Cookie) error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}
	defer conn.Close()

	sessionClearQuery := `
	delete from adm.activesessions
	where token = $1
	`
	_, err = conn.Exec(sessionClearQuery, cookie.Value)
	if err != nil {
		return err
	}

	return nil
}