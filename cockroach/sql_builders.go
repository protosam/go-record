package record

import (
	"reflect"
	"strconv"
	"strings"
)

// Deletes table from database (DROP TABLE)
func (self *Model) DeleteTable() error {
	generated_sql := "DROP TABLE " + self.TableName + ";"
	stmt, err := DB.Prepare(generated_sql)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec()
	return err
}

// Creates the table and indexes defined, if it does not exists.
func (self *Model) CreateTable() error {

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

// Generates SELECT statements to be executed.
func (self *Model) gen_select() string {
	var selected_fields []string

	for field_name, db_field_name := range self.db_field_name {
		select_frag := self.table_association[field_name] + "." + db_field_name + " as \"" + self.table_association[field_name] + "." + db_field_name + "\""
		selected_fields = append(selected_fields, select_frag)
	}

	generated_sql := "SELECT " + strings.Join(selected_fields, ", ") + " FROM " + self.TableName

	if self.JoinFrag != "" {
		generated_sql += " " + self.JoinFrag
	}

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

// Generates INSERT statements to be executed.
func (self *Model) gen_insert(fields, input_serials []string) string {
	return "INSERT INTO " + self.TableName + " (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(input_serials, ", ") + ") RETURNING " + self.primary_key + ";"
}

// Returns DATABASE's defined datatype for a field in the structure.
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
