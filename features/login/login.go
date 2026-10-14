package login

import (
	"crypto/subtle"
	"database/sql"
	"hash/fnv"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func CreateDatabase() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}

	var queries []string
	file, err := os.ReadFile("/src/connection/db.sql")
	if err != nil {
		return err
	}

	queries = append(queries, strings.Split(string(file), "----")...)

	tx, err := conn.Begin()
	if err != nil {
		return err
	}

	for _, query := range queries {
		_, err := tx.Exec(query)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()

}

func LoginValidator(cookie string) (bool, time.Time, error) {
	var expire time.Time
	conn, err := connection.Db()
	if err != nil {
		return false, expire, err
	}

	query := `
	select expire
	from frontdesk.activesessions
	where token = $1 and expire >= now() `
	err = conn.QueryRow(query, cookie).Scan(&expire)
	if err == sql.ErrNoRows {
		return false, expire, nil
	}
	if err != nil {
		return false, expire, err
	}

	return true, expire, nil
}

// pushes the session another 24h from now
func RefreshSession(token string) (time.Time, error) {
	var expire time.Time
	conn, err := connection.Db()
	if err != nil {
		return expire, err
	}

	query := `
	update frontdesk.activesessions
		SET expire = $2
	where
		token = $1
	RETURNING expire`

	err = conn.QueryRow(query, token, time.Now().Add(24*time.Hour)).Scan(&expire)
	if err != nil {
		return expire, err
	}

	return expire, nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// returns if it matches and if it is one of the old fnv hashes that need to become bcrypt
func verifyPassword(stored string, password string) (bool, bool) {
	if strings.HasPrefix(stored, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil, false
	}

	// fnv was a terrible idea, but some people already have users with it
	hashPass := fnv.New32a()
	hashPass.Write([]byte(password))
	legacy := strconv.FormatUint(uint64(hashPass.Sum32()), 10)
	ok := subtle.ConstantTimeCompare([]byte(stored), []byte(legacy)) == 1
	return ok, ok
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
		Name:    "front_desk_awesome_cookie",
		Value:   uuid.New().String(),
		Domain:  os.Getenv("DOMAIN_NAME"),
		Expires: time.Now().Add(time.Hour * 24),
	}

	conn, err := connection.Db()
	if err != nil {
		return nil, err
	}

	query := `
	select id, password
	from frontdesk.users
	where
		username = $1 `
	res, err := conn.Query(query, u.UserName)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	found := false
	needsRehash := false
	for res.Next() {
		var id int
		var stored string
		err = res.Scan(&id, &stored)
		if err != nil {
			return nil, err
		}
		ok, rehash := verifyPassword(stored, u.Password)
		if ok {
			u.Id = id
			found = true
			needsRehash = rehash
			break
		}
	}

	if !found {
		session.Valid = false
		return &session, nil
	}

	sessionClearQuery := `
	delete from frontdesk.activesessions
	where userid = $1 and expire < now()
	`
	sessionCreateQuery := `
	insert into frontdesk.activesessions
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

	if needsRehash {
		newHash, err := hashPassword(u.Password)
		if err != nil {
			tran.Rollback()
			return nil, err
		}
		_, err = tran.Exec(`update frontdesk.users set password = $1 where id = $2`, newHash, u.Id)
		if err != nil {
			tran.Rollback()
			return nil, err
		}
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
	hashPass, err := hashPassword(u.Password)
	if err != nil {
		return *u, err
	}

	conn, err := connection.Db()
	if err != nil {
		return *u, err
	}

	query := `
	insert into frontdesk.users (username, password)
	values ($1,$2)
	RETURNING id, username`

	err = conn.QueryRow(query, u.UserName, hashPass).Scan(&u.Id, &u.UserName)
	if err != nil {
		return *u, err
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

	var count int
	query := `
	select count(id)
	from frontdesk.users `
	res, err := conn.Query(query)
	if err != nil {
		return err
	}
	defer res.Close()

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

	sessionClearQuery := `
	delete from frontdesk.activesessions
	where token = $1
	`
	_, err = conn.Exec(sessionClearQuery, cookie.Value)
	if err != nil {
		return err
	}

	return nil
}
