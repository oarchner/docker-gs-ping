package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cockroachdb/cockroach-go/v2/crdb"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
)

func main() {

	fmt.Println("Starting ...")

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	db, err := initStore()
	if err != nil {
		log.Fatalf("failed to initialize the store: %s", err)
	}
	defer db.Close()
	fmt.Println("db initialzed")

	e.GET("/", func(c echo.Context) error {
		return rootHandler(db, c)
	})

	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct{ Status string }{Status: "OK"})
	})

	e.POST("/send", func(c echo.Context) error {
		return sendHandler(db, c)
	})

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	fmt.Printf("Started on port:%v\n", httpPort)
	e.Logger.Fatal(e.Start(":" + httpPort))

}

type Message struct {
	Value string `json:"value"`
}

func initStore() (*sql.DB, error) {

	fmt.Println("Init db store ...")

	pgConnString := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		os.Getenv("PGHOST"),
		os.Getenv("PGPORT"),
		os.Getenv("PGDATABASE"),
		os.Getenv("PGUSER"),
		os.Getenv("PGPASSWORD"),
	)

	fmt.Printf("Conn:%s", pgConnString)

	var (
		db  *sql.DB
		err error
	)

	if db, err = sql.Open("postgres", pgConnString); err != nil {
		return nil, err
	}

	fmt.Println("db opened ...")

	if _, err := db.Exec(
		"CREATE TABLE IF NOT EXISTS message (value TEXT PRIMARY KEY)"); err != nil {
		return nil, err
	}

	return db, nil
}

func rootHandler(db *sql.DB, c echo.Context) error {
	r, err := countRecords(db)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, err.Error())
	}
	return c.HTML(http.StatusOK, fmt.Sprintf("Hello, Docker! (%d)\n", r))
}

func sendHandler(db *sql.DB, c echo.Context) error {

	m := &Message{}

	if err := c.Bind(m); err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	err := crdb.ExecuteTx(context.Background(), db, nil,
		func(tx *sql.Tx) error {
			_, err := tx.Exec(
				"INSERT INTO message (value) VALUES ($1) ON CONFLICT (value) DO UPDATE SET value = excluded.value",
				m.Value,
			)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, err)
			}
			return nil
		})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, m)
}

func countRecords(db *sql.DB) (int, error) {

	rows, err := db.Query("SELECT COUNT(*) FROM message")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
		rows.Close()
	}

	return count, nil
}

func IntMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
