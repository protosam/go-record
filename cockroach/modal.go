package record

import (
	"fmt"
	"log"
	"reflect"
	"strings"
)

// Base super struct to be embedded in models.
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

// Extracts information about structure to help build SQL statements
// and manipulate the structure using reflect.
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

// Returns array of pointers to the fields in the table structure.
// Allows easy rows.Scan() to take place.
func (self *Model) get_selector_associations(cols ...string) []interface{} {
	var references []interface{}
	for _, selector := range cols {
		references = append(references, reflect.ValueOf(self.Model).Elem().FieldByName(self.selector_associations[selector]).Addr().Interface())
	}

	return references
}
