package record

import (
	"database/sql"
	"regexp"

	_ "github.com/lib/pq"
)

// DB connection for queries and such.
var DB *sql.DB

// Used for camel_case to snake_case function
var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

/**************** Initialization Functions ****************/
func Connect(conn_str string) error {
	var err error
	conn_str = "postgresql://" + conn_str
	DB, err = sql.Open("postgres", conn_str)
	return err
}

func Close() {
	DB.Close()
}
