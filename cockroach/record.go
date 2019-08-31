package record

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

// DB connection for queries and such.
var DB *sql.DB

// Used for camel_case to snake_case function
var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

type Model struct {
	Model                  interface{}
	primary_key            string
	primary_key_field_name string
	TableName              string
	JoinFrag               string

	//////////////////////////////////////
	// Model description
	// .................
	// Used to keep track of struck fieldname to database column name
	db_field_name map[string]string
	// Used to keep track of which table each struct field belongs to
	table_association map[string]string
	// Used to know for db_opts.
	options map[string]map[string]string
	// Used to backtrack tablename.column to stuct fieldname
	selector_associations map[string]string

	// Used to keep track of values so we can handle row updates in .Save()
	value_tracker map[string]interface{}

	// Select builder
	where_frag  string
	limit_frag  int
	offset_frag int
	order_frags []string
}

// Helper function to pull in Model struct data
func (self *Model) Init(obj interface{}) {
	self.Model = obj
	self.Extract()
	self.update_value_tracker()
}

// Update the value tacker
func (self *Model) update_value_tracker() {
	self.value_tracker = make(map[string]interface{})

	for field_name, _ := range self.db_field_name {
		field := reflect.ValueOf(self.Model).Elem().FieldByName(field_name)
		self.value_tracker[field_name] = field.Interface()
	}
}

// Check if a field has changed, by field name
func (self *Model) FieldChanged(field_name string) bool {
	field := reflect.ValueOf(self.Model).Elem().FieldByName(field_name)
	if field.IsValid() {
		new_value, ok := self.value_tracker[field_name]
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

func (self *Model) Extract() {
	self.db_field_name = make(map[string]string)
	self.table_association = make(map[string]string)
	self.options = make(map[string]map[string]string)
	self.selector_associations = make(map[string]string)

	s := reflect.ValueOf(self.Model).Elem()
	stype := reflect.TypeOf(self.Model).Elem()
	typeOfT := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		field, _ := stype.FieldByName(typeOfT.Field(i).Name)

		// Ensure the f.Type() is a string... lazymode...
		field_type := fmt.Sprintf("%s", f.Type())

		// Skip embedded types
		if !strings.Contains(field_type, ".") {
			field_name := typeOfT.Field(i).Name
			db_field_name := self.format_field_name(field_name)

			db_opts := field.Tag.Get("db_opts")
			if db_opts != "-" {
				self.db_field_name[field_name] = db_field_name
				self.options[field_name] = make(map[string]string)

				for _, opt := range strings.Split(db_opts, ";") {
					if strings.Contains(opt, ":") {
						sep := strings.Split(opt, ":")
						self.options[field_name][strings.ToLower(strings.TrimSpace(sep[0]))] = strings.TrimSpace(sep[1])
					} else {
						self.options[field_name][strings.ToLower(strings.TrimSpace(opt))] = "-"
					}
				}

				// Alternative field names...
				if _, ok := self.options[field_name]["column"]; ok {
					self.db_field_name[field_name] = self.options[field_name]["column"]
				}

				// Set primary key name
				if _, ok := self.options[field_name]["primary_key"]; ok {
					if self.primary_key != "" {
						log.Println("WARNING! Multiple primary keys assigned for (GoRecord only supports 1):", self.TableName)
					}

					self.primary_key = self.db_field_name[field_name]
					self.primary_key_field_name = field_name
				}

				// Set table association
				if _, ok := self.options[field_name]["table"]; ok {
					self.table_association[field_name] = self.options[field_name]["table"]
				} else {
					self.table_association[field_name] = self.TableName
				}

				// Set selector association
				association := self.table_association[field_name] + "." + self.db_field_name[field_name]
				self.selector_associations[association] = field_name
			}
		}
	}

}

// Handles field name formatting.
func (self *Model) format_field_name(field_name string) string {
	//field_opts := self.GetOpts(field_name)
	str := field_name

	// snake case it... we assume this if no column name was specified
	str = matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	str = matchAllCap.ReplaceAllString(str, "${1}_${2}")
	str = strings.ReplaceAll(str, "__", "_")
	str = strings.ToLower(str)

	return str
}

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

	for field_name, db_field_name := range self.db_field_name {

		// Make the col string with the column name
		col := db_field_name + " "

		// If the dev specifies "raw", they need to write everything about the col
		if _, ok := self.options[field_name]["raw"]; ok {
			col += self.options[field_name]["raw"]
		} else {
			// Add it's DB data type
			// We have to use the SERIAL data type for AUTO_INCREMENT
			if _, ok := self.options[field_name]["auto_increment"]; ok {
				col += "SERIAL"
			} else if opt, ok := self.options[field_name]["type"]; ok { // Check if a specific type was specified
				col += opt
			} else { // Default to model check
				col += self.field_db_datatype(field_name)
			}

			// Is this a primary key?
			if _, ok := self.options[field_name]["primary_key"]; ok {
				col += " PRIMARY KEY"
			}

			// Unique?
			if _, ok := self.options[field_name]["unique"]; ok {
				col += " UNIQUE"
			}

			// Not null?
			if _, ok := self.options[field_name]["not null"]; ok {
				col += " NOT NULL"
			}

			// Default value?
			if opt, ok := self.options[field_name]["default"]; ok {
				col += " DEFAULT " + opt
			}

			// Add to indexes lines.
			if opt, ok := self.options[field_name]["index"]; ok {
				if opt == "-" {
					opt = self.TableName + "_" + db_field_name + "_idx"
				}
				indexes = append(indexes, "CREATE INDEX IF NOT EXISTS "+opt+" ON "+self.TableName+" ("+db_field_name+");")
			}
		}

		columns = append(columns, col)
	}

	create_table := "CREATE TABLE IF NOT EXISTS " + self.TableName + " (\n    "
	create_table += strings.Join(columns, ",\n    ")
	create_table += "\n);"

	var generated_sql []string
	generated_sql = append(generated_sql, create_table)
	generated_sql = append(generated_sql, indexes...)

	for _, entry := range generated_sql {
		stmt, err := DB.Prepare(entry)
		if err != nil {
			return err
		}
		defer stmt.Close()

		if _, err = stmt.Exec(); err != nil {
			return err
		}
	}
	return nil

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

// Insert/Update
func (self *Model) Save() error {
	var err error
	// Determine if we need to insert a new row or update an existing row.
	if self.isset(self.primary_key_field_name) {
		err = self.update()
	} else {
		err = self.insert()
	}

	if err != nil {
		return err
	}

	self.update_value_tracker()

	return nil
}

// Create
func (self *Model) insert() error {

	var fields []string
	var values []interface{}
	var input_serials []string
	for field_name, db_field_name := range self.db_field_name {
		if self.table_association[field_name] == self.TableName && db_field_name != self.primary_key {
			fields = append(fields, db_field_name)
			values = append(values, reflect.ValueOf(self.Model).Elem().FieldByName(field_name).Interface())
			input_serials = append(input_serials, "$"+strconv.Itoa(len(input_serials)+1))
		}
	}

	generated_sql := "INSERT INTO " + self.TableName + " (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(input_serials, ", ") + ") RETURNING " + self.primary_key + ";"

	stmt, err := DB.Prepare(generated_sql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Handle running and returning primary key based on the data type of the pk...

	// Get pk field
	field := reflect.ValueOf(self.Model).Elem().FieldByName(self.primary_key_field_name)

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
	var set_fields []string
	var values []interface{}
	for field_name, db_field_name := range self.db_field_name {
		if self.table_association[field_name] == self.TableName && self.FieldChanged(field_name) {
			set_fields = append(set_fields, db_field_name+" = $"+strconv.Itoa(len(set_fields)+1))
			values = append(values, reflect.ValueOf(self.Model).Elem().FieldByName(field_name).Interface())
			changes_found = true
		}
	}

	// If nothing was changed, no need for running an UPDATE statement...
	if !changes_found {
		return nil
	}

	// Add primary key value
	values = append(values, reflect.ValueOf(self.Model).Elem().FieldByName(self.primary_key_field_name).Interface())

	// Generate update statement
	generated_sql := "UPDATE " + self.TableName + " SET " + strings.Join(set_fields, ",") + " WHERE " + self.primary_key + " = $" + strconv.Itoa(len(values)) + ";"

	// Run the query.
	_, err := DB.Exec(generated_sql, values...)
	if err != nil {
		return err
	}
	return nil
}

// Delete
func (self *Model) Delete() error {
	generated_sql := "DELETE FROM " + self.TableName + " WHERE " + self.primary_key + " = $1;"

	// Get pk field
	field := reflect.ValueOf(self.Model).Elem().FieldByName(self.primary_key_field_name)

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

func (self *Model) Fetch(vals ...interface{}) error {

	generated_sql := self.gen_select()

	rows, err := DB.Query(generated_sql, vals...)
	if err != nil {
		return err
	}

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	if rows.Next() {
		references := self.get_selector_associations(cols...)

		if err := rows.Scan(references...); err != nil {
			return err
		}

		// Update tracked data...
		self.update_value_tracker()
	}

	return nil
}

func (self *Model) Each(fn func(error) bool, vals ...interface{}) {
	generated_sql := self.gen_select()

	rows, err := DB.Query(generated_sql, vals...)
	if err != nil {
		fn(err)
		return
	}

	cols, err := rows.Columns()
	if err != nil {
		fn(err)
		return
	}

	defer rows.Close()
	for rows.Next() {
		references := self.get_selector_associations(cols...)

		if err := rows.Scan(references...); err != nil {
			fn(err)
			return
		}

		// Update tracked data...
		self.update_value_tracker()

		// if fn(error)bool returns false, break the rows.Next() loop.
		if !fn(nil) {
			return
		}
	}
}

// Returns array of pointers to the fields in the table structure.
// Allows easy rows.Scan() to take place.
func (self *Model) get_selector_associations(cols ...string) []interface{} {
	var references []interface{}
	for _, selector := range cols {
		references = append(references, reflect.ValueOf(self.Model).Elem().FieldByName(self.selector_associations[selector]).Addr().Interface())
	}

	return references
}

func (self *Model) gen_select() string {
	var selected_fields []string

	for field_name, db_field_name := range self.db_field_name {
		select_frag := self.table_association[field_name] + "." + db_field_name + " as \"" + self.table_association[field_name] + "." + db_field_name + "\""
		selected_fields = append(selected_fields, select_frag)
	}

	generated_sql := "SELECT " + strings.Join(selected_fields, ", ") + " FROM " + self.TableName

	/* TODO: NEED TO BUILD JOIN FRAGMENT....
	Joins should be built from tags...

	EXAMPLE: join t2 on t1.t1_id = t2.t1_id
	*/

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

	return generated_sql + ";"
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
