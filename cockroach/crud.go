package record

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
)

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

	generated_sql := self.gen_insert(fields, input_serials)

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

// TODO: Need to somehow make more modular SQL syntax generation for this.
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
