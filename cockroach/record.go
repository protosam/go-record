package record

import (
	"database/sql"

	_ "github.com/lib/pq"
)

// DB connection for queries and such.
var DB *sql.DB

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
