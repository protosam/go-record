package record

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"database/sql"

	_ "github.com/lib/pq"
)

// DB connection for queries and such.
var DB *sql.DB

// Used for camel_case to snake_case function
var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

type Model struct {
	Model      interface{}
	PrimaryKey string
	TableName  string
	RowCount   int

	// Sanity
	StructPkName string

	// Extracted
	Fields    []string
	Values    map[string]interface{}
	Types     map[string]string
	Tags      map[string]string
	DB_Fields map[string]string

	// Select builder
	where_frag  string
	limit_frag  int
	offset_frag int
	order_frags []string
}

/**************** OO Magic Function ****************/

// Helper function to pull in Model struct data
func (self *Model) Init(obj interface{}) {
	self.Model = obj
	self.Extract()
}

/**************** CRUD Operation Functions ****************/

// Insert/Update
func (self *Model) Save() error {
	var err error
	// Determine if we need to insert a new row or update an existing row.
	if self.isset(self.StructPkName) {
		err = self.update()
	} else {
		err = self.insert()
	}

	if err != nil {
		return err
	}

	// Update extracted data...
	self.Extract()

	return nil
}

// Create
func (self *Model) insert() error {
	db_pk_name := self.format_field_name(self.StructPkName)

	var field_selector []string
	var input_serials []string
	var values []interface{}
	for i, field := range self.Fields {
		// Need to filter primary key if we want it or not...
		db_field_name := self.format_field_name(field)
		if db_field_name != db_pk_name {
			field_selector = append(field_selector, db_field_name)
			input_serials = append(input_serials, "$"+strconv.Itoa(i))
			values = append(values, reflect.ValueOf(self.Model).Elem().FieldByName(field).Interface())
		}
	}

	generated_sql := "INSERT INTO " + self.TableName + " (" + strings.Join(field_selector, ", ") + ") VALUES (" + strings.Join(input_serials, ", ") + ") RETURNING " + db_pk_name + ";"

	stmt, err := DB.Prepare(generated_sql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Handle running and returning PrimaryKey based on the data type of the pk...

	// Get pk field
	field := reflect.ValueOf(self.Model).Elem().FieldByName(self.StructPkName)

	// Switch action based on pk kind()...
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var return_id int64
		err = stmt.QueryRow(values...).Scan(&return_id)
		if err != nil {
			return err
		}

		field.SetInt(return_id)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var return_id uint64
		err = stmt.QueryRow(values...).Scan(&return_id)
		if err != nil {
			return err
		}

		field.SetUint(return_id)
	case reflect.String:
		var return_id string
		err = stmt.QueryRow(values...).Scan(&return_id)
		if err != nil {
			return err
		}

		field.SetString(return_id)
	default:
		// Do nothing
		return errors.New("Primary key problem from Model.Insert()")
	}

	return nil
}

// Update
func (self *Model) update() error {

	changes_found := false
	var field_selector []string
	var values []interface{}
	i := 1 // Have to initiate i for updates due to field selector being arbitrary
	for _, field := range self.Fields {
		if self.FieldChanged(field) {
			changes_found = true
			field_selector = append(field_selector, self.format_field_name(field)+" = $"+strconv.Itoa(i))
			values = append(values, reflect.ValueOf(self.Model).Elem().FieldByName(field).Interface())
			i++
		}
	}

	// If nothing was changed, no need for running an UPDATE statement...
	if !changes_found {
		return nil
	}

	values = append(values, reflect.ValueOf(self.Model).Elem().FieldByName(self.StructPkName).Interface())

	generated_sql := "UPDATE " + self.TableName + " SET " + strings.Join(field_selector, ",") + " WHERE " + self.format_field_name(self.StructPkName) + " = $" + strconv.Itoa(len(values)) + ";"

	_, err := DB.Exec(generated_sql, values...)
	if err != nil {
		return err
	}

	return nil
}

// Delete
func (self *Model) Delete() error {
	generated_sql := "DELETE FROM " + self.TableName + " WHERE " + self.format_field_name(self.StructPkName) + " = $1;"

	// Get pk field
	field := reflect.ValueOf(self.Model).Elem().FieldByName(self.StructPkName)

	_, err := DB.Exec(generated_sql, field.Interface())
	if err != nil {
		return err
	}

	return nil
}

// Retreive
func (self *Model) Where(sql string) *Model {
	self.where_frag = sql

	return self
}
func (self *Model) Limit(i int) *Model {
	self.limit_frag = i
	return self
}
func (self *Model) Start(i int) *Model {
	self.offset_frag = i
	return self
}

func (self *Model) Descending(field_name string) *Model {
	ofrag := self.format_field_name(field_name) + " DESC"
	self.order_frags = append(self.order_frags, ofrag)
	return self
}

func (self *Model) Ascending(field_name string) *Model {
	ofrag := self.format_field_name(field_name) + " ASC"
	self.order_frags = append(self.order_frags, ofrag)
	return self
}

func (self *Model) gen_query_sql() string {
	generated_sql := "SELECT * FROM " + self.TableName
	if self.where_frag != "" {
		generated_sql += " WHERE " + self.where_frag
	}
	if len(self.order_frags) > 0 {
		generated_sql += " ORDER BY " + strings.Join(self.order_frags, ", ")
	}

	if self.limit_frag != 0 {
		generated_sql += " LIMIT " + strconv.Itoa(self.limit_frag)
	}
	if self.offset_frag != 0 {
		generated_sql += " OFFSET " + strconv.Itoa(self.offset_frag)
	}

	return generated_sql
}

// Just fetch the first result....
func (self *Model) Fetch(vals ...interface{}) error {
	generated_sql := self.gen_query_sql()

	rows, err := DB.Query(generated_sql, vals...)
	if err != nil {
		return err
	}

	if rows.Next() {
		var references []interface{}
		for _, field := range self.Fields {
			references = append(references, reflect.ValueOf(self.Model).Elem().FieldByName(field).Addr().Interface())
		}

		if err := rows.Scan(references...); err != nil {
			return err
		}

		// Update extracted data...
		self.Extract()
	}

	return nil
}

// Iterate all results and run them through an anonymous function.
// If the function returns FALSE, break the loop.
func (self *Model) Each(fn func(error) bool, vals ...interface{}) {
	generated_sql := self.gen_query_sql()

	rows, err := DB.Query(generated_sql, vals...)
	if err != nil {
		fn(err)
		return
	}

	defer rows.Close()
	for rows.Next() {
		var references []interface{}
		for _, field := range self.Fields {
			references = append(references, reflect.ValueOf(self.Model).Elem().FieldByName(field).Addr().Interface())
		}

		if err := rows.Scan(references...); err != nil {
			fn(err)
			return
		}

		// Update extracted data...
		self.Extract()

		// if fn(error)bool returns false, break the rows.Next() loop.
		if !fn(nil) {
			return
		}
	}
}

// Return the total number of results.
func (self *Model) Count() int {
	return 0
}

/**************** Utility Functions ****************/

// Handles field name formatting.
func (self *Model) format_field_name(field_name string) string {
	field_opts := self.GetOpts(field_name)
	str := field_name

	// check tags for alternative column name
	if db_field_name, ok := field_opts["column"]; ok {
		return db_field_name
	}

	// snake case it... we assume this if no column name was specified
	str = matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	str = matchAllCap.ReplaceAllString(str, "${1}_${2}")
	str = strings.ReplaceAll(str, "__", "_")
	str = strings.ToLower(str)

	return str
}

// Determine if a field was set or not (mainly used for primary key checking)
func (self *Model) isset(field_name string) bool {
	field := reflect.ValueOf(self.Model).Elem().FieldByName(field_name)
	if field.IsValid() {
		// return comparison based on field kind. The new value is expected to be the same type...
		switch field.Kind() {
		case reflect.Int:
			return field.Interface().(int) != 0
		case reflect.Int8:
			return field.Interface().(int8) != 0
		case reflect.Int16:
			return field.Interface().(int16) != 0
		case reflect.Int32:
			return field.Interface().(int32) != 0
		case reflect.Int64:
			return field.Interface().(int64) != 0
		case reflect.Uint:
			return field.Interface().(uint) != 0
		case reflect.Uint8:
			return field.Interface().(uint8) != 0
		case reflect.Uint16:
			return field.Interface().(uint16) != 0
		case reflect.Uint32:
			return field.Interface().(uint32) != 0
		case reflect.Uint64:
			return field.Interface().(uint64) != 0
		case reflect.String:
			return field.Interface().(string) != ""
		}
	}
	return false
}

// Check if a field has changed, by field name
func (self *Model) FieldChanged(field_name string) bool {
	field := reflect.ValueOf(self.Model).Elem().FieldByName(field_name)
	if field.IsValid() {
		new_value, ok := self.Values[field_name]
		if !ok {
			return false
		}

		// return comparison based on field kind. The new value is expected to be the same type...
		switch field.Kind() {
		case reflect.Bool:
			return new_value.(bool) != field.Interface().(bool)
		case reflect.Int:
			return new_value.(int) != field.Interface().(int)
		case reflect.Int8:
			return new_value.(int8) != field.Interface().(int8)
		case reflect.Int16:
			return new_value.(int16) != field.Interface().(int16)
		case reflect.Int32:
			return new_value.(int32) != field.Interface().(int32)
		case reflect.Int64:
			return new_value.(int64) != field.Interface().(int64)
		case reflect.Uint:
			return new_value.(uint) != field.Interface().(uint)
		case reflect.Uint8:
			return new_value.(uint8) != field.Interface().(uint8)
		case reflect.Uint16:
			return new_value.(uint16) != field.Interface().(uint16)
		case reflect.Uint32:
			return new_value.(uint32) != field.Interface().(uint32)
		case reflect.Uint64:
			return new_value.(uint64) != field.Interface().(uint64)
		case reflect.Float32:
			return new_value.(float32) != field.Interface().(float32)
		case reflect.Float64:
			return new_value.(float64) != field.Interface().(float64)
		case reflect.String:
			return new_value.(string) != field.Interface().(string)
		default:
			// Do nothing
		}
	}
	return false
}

// Denature's object data from self.Model
func (self *Model) Extract() {
	// Initialize maps
	self.Fields = nil
	self.Values = make(map[string]interface{})
	self.Types = make(map[string]string)
	self.Tags = make(map[string]string)
	self.DB_Fields = make(map[string]string)

	s := reflect.ValueOf(self.Model).Elem()
	stype := reflect.TypeOf(self.Model).Elem()
	typeOfT := s.Type()

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		field, _ := stype.FieldByName(typeOfT.Field(i).Name)

		// Ensure the f.Type() is a string... lazymode...
		field_type := fmt.Sprintf("%s", f.Type())

		// We're not bothering with embedded types...
		if !strings.Contains(field_type, ".") {
			field_name := typeOfT.Field(i).Name

			db_opts := field.Tag.Get("db_opts")
			if db_opts != "-" {
				self.Tags[field_name] = db_opts
				self.Types[field_name] = typeOfT.Field(i).Name
				self.Values[field_name] = f.Interface()
				self.Fields = append(self.Fields, field_name)
			}

			// self.StructPkName needs to have the REAL field name (sometimes you just have to save people from themselves)
			if field_name == self.PrimaryKey {
				self.StructPkName = field_name
			} else if self.format_field_name(field_name) == self.format_field_name(self.PrimaryKey) {
				self.StructPkName = field_name
			}
		}
	}
}

// Get field db_opts that were defined in the model.
func (self *Model) GetOpts(field_name string) map[string]string {
	options := make(map[string]string)
	for _, opt := range strings.Split(self.Tags[field_name], ";") {
		if strings.Contains(opt, ":") {
			sep := strings.Split(opt, ":")
			options[strings.ToLower(strings.TrimSpace(sep[0]))] = strings.TrimSpace(sep[1])
		} else {
			options[strings.ToLower(strings.TrimSpace(opt))] = "-"
		}
	}

	return options
}

/*
CREATE TABLE IF NOT EXISTS users (
	user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	username STRING,
	email STRING,
	password STRING,
	hashword STRING,
	login_token STRING,
	last_touch INT,
	money INT,
	exp INT,
	admin BOOL
);

CREATE INDEX IF NOT EXISTS users_username_idx ON users (username);

*/
func (self *Model) field_db_datatype(field_name string) string {
	field := reflect.ValueOf(self.Model).Elem().FieldByName(field_name)
	if field.IsValid() {
		// return comparison based on field kind. The new value is expected to be the same type...
		switch field.Kind() {
		case reflect.Bool:
			return "BOOL"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return "INT"
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64: // maybe one day cockroach will support UINT? :(
			return "INT"
		case reflect.Float32, reflect.Float64:
			return "FLOAT"
		case reflect.String:
			return "STRING"
		}
	}
	return "STRING"
}

func (self *Model) AutoMigrate() error {

	var columns []string
	var indexes []string
	for _, field := range self.Fields {
		field_opts := self.GetOpts(field)

		// Make the col string with the column name
		col := self.format_field_name(field) + " "

		// If the dev specifies "raw", they need to write everything about the col
		if _, ok := field_opts["raw"]; ok {
			col += field_opts["raw"]
		} else {
			// Add it's DB data type
			// We have to use the SERIAL data type for AUTO_INCREMENT
			if _, ok := field_opts["auto_increment"]; ok {
				col += "SERIAL"
			} else if opt, ok := field_opts["type"]; ok { // Check if a specific type was specified
				col += opt
			} else { // Default to model check
				col += self.field_db_datatype(field)
			}

			// Is this a primary key?
			if self.format_field_name(self.StructPkName) == self.format_field_name(field) {
				col += " PRIMARY KEY"
			}

			// Unique?
			if _, ok := field_opts["not null"]; ok {
				col += " UNIQUE"
			}

			// Not null?
			if _, ok := field_opts["not null"]; ok {
				col += " NOT NULL"
			}

			// Default value?
			if opt, ok := field_opts["default"]; ok {
				col += " DEFAULT " + opt
			}

			// Add to indexes lines.
			if opt, ok := field_opts["index"]; ok {
				if opt == "-" {
					opt = self.TableName + "_" + col + "_idx"
				}
				indexes = append(indexes, "CREATE INDEX IF NOT EXISTS "+opt+" ON users ("+col+");")
			}
		}

		columns = append(columns, col)
	}

	generated_sql := "CREATE TABLE IF NOT EXISTS " + self.TableName + " (\n    "
	generated_sql += strings.Join(columns, ",\n    ")
	generated_sql += "\n);\n"
	generated_sql += strings.Join(indexes, ",\n")

	stmt, err := DB.Prepare(generated_sql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec()

	return err
}

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
