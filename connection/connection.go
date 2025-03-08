package connection

import (
	"fmt"
	"os"
	"database/sql"
	_ "github.com/lib/pq"
)

func Db() (*sql.DB, error) {
	HOST := os.Getenv("PG_HOST")
	PORT := os.Getenv("PG_PORT")
	USER := os.Getenv("PG_USER")
	PASSWORD := os.Getenv("PG_PASSWORD")
	DBNAME := os.Getenv("PG_DBNAME")

	conn, err := sql.Open("postgres",
		fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			HOST, PORT, USER, PASSWORD, DBNAME),
	)

	if err != nil {
		return nil, err
	}
	err = conn.Ping()

	return conn, err
}
