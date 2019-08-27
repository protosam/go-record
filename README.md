# Go-Record
An Active Record pattern CRUD tool for dealing with databases.

#### What's Working
`Go-Record/cockroach` is in a useable state. It can generate tables and perform CRUD operations against database tables.

Right now this is still in development and there may be changes to how the library is used, based on feedback and things I encounter myself as I use it. I think this is at a good starting point though.

#### What's Next
Bindings still need to be made for the following SQL databases. If you want to contribute to one of these, please feel free to make a pull request.
```
Go-Records/mysql
Go-Records/postgresql
Go-Records/mssql
Go-Records/sqlite
```

#### Defining a Model
You will need to define your database model like so:
```
package main

import (
	// Import the cockroach record package (it's still "record")
	"github.com/protosam/record/cockroach"
)

// Model struct
type Users struct {
	// Embed the record.Model type
	record.Model

	// Define DB fields (Note: Tags explained in next section)
	User_Id     string `db_opts:"type:UUID;default:gen_random_uuid()"`
	Username    string
	Email       string
	Password    string
	CoolPoints  int
	Admin       bool
}


// Constructor so that we can use record.Model{} methods
func (self Users) New() *Users {
	self.TableName = "users"
	self.PrimaryKey = "User_Id"

	// Required to ensure that embedded record.Model
	// can work with our model.
	self.Init(&self)
	return &self
}

```

#### Tagging
The `db_opts` tag is parsed by the `record` library. These options allow a bit more flexibility in managing what your database tables look like. The option names aren't case sensitive.

```
COLUMN
Specify the real column name to be used instead of the camel case conversion that record.Model uses.
Example: `db_opts:"type:int;column:real_user_id"`

TYPE
Specify the real underlying datatype that the database is using, instead of having record.Model guestimate it.
Example: `db_opts:"type:int"`

UNIQUE
Specifies column as unique. This is a toggle option.
Examples: `db_opts:"type:int;unique"` / `db_opts:"unique;type:int"`

DEFAULT
Specify the default value of your column.
Example: `db_opts:"type:uuid;column:real_user_id;default:gen_random_uuid()"`

NOT NULL
Specifies column as NOT NULL. This is a toggle option.
Example: `db_opts:"unique;not null;type:int"`

AUTO_INCREMENT
Sets column as an automatic incrementing column. This is a toggle option. In Cockroach DB it sets the data type to SERIAL.
Example: `db_opts:"Auto_increment;type:int"`

INDEX
Create index with or without name, same name creates composite indexes
Examples: `db_opts:"type:int;index"` / `db_opts:"index:some_idx_name;type:int"`


RAW
Choose your own DB options manually that get placed in the create statement.
Example: `db_opts:"raw:UUID PRIMARY KEY DEFAULT gen_random_uuid()"`

-
Ignore this field...
Example: `db_opts:"-"`
```

### Basic CRUD

#### Create a Table
This will take your model structure and make a table schema.
```
err := Users{}.New().AutoMigrate();
if err != nil {
	log.Fatal(err)
}
```

#### Create Database Entry (Insert)
Below is a basic example of creating a new row in the model's table.
```
u := Users{Username: "foo", Email: "noreply@github.com", Password: "secure_me_plz", CoolPoints: "1000000", Admin: true}.New()
if err := u.Save(); err != nil {
	log.Fatal(err)
}
new_user_id := u.User_Id
fmt.Println("New User Id:", u.User_Id)
```

#### Iterate all Selected Rows
To retreive more than one row, you will need to use `.Each( func(error)bool, vars... )`. You can chain `Where()` before it along with other chainable methods that will be discussed later.

Inside your anonymous function the parent structure gets populated with data as it goes. You can do `Save()` and `Delete()` operations safely while iterating rows. If you want to double up iterating loops through the database, it is recommended that you use a separate variable with a new `YourModel{}.New()`.
```
u := Users{}.New() // We get a fresh &User{} object so no prior SQL clauses exist.
u.Each(func(err error) bool {
	// Note, u.Save() and u.Delete() can run here
	fmt.Println(u.Username, "-", u.CoolPoints)
	return true
})
```


#### Fetch the First Result + Where()
Getting data back from the database can get a bit complicated, so we're starting with just the `Fetch` method. `Fetch` is a variadic function that takes 1 argument for every defined value in your `Where`'s SQL fragment. Note that the columns referenced in the SQL frament MUST be as they end up in the database, not the struct fields. This was due to a compromise for flexibility.

Once `.Fetch()` is called, it will fill the model structure with the data of the first row returned.
```
u := Users{}.New()
u.Where("user_id = $1").Fetch(new_user_id)
fmt.Println(u.Username)
```

#### Each is Variadic too
Like fetch, when you call `.Each()`, after the anonymous function, you can add values to fill in `Where()` clauses
```
u := Users{}.New() // We get a fresh &User{} object so no prior SQL clauses exist.
u.Where("cool_points > $1").Each(func(err error) bool {
	// Note, u.Save() and u.Delete() can run here
	fmt.Println(u.Username, "-", u.CoolPoints)
	return true
}, 100)
```


#### Update a Record
```
Updating records is super simple. Once you're using a retreived record, you just use `.Save()`. It relies on the primary key along with some convoluted tracking done behind the scenes to run an `UPDATE` query that only updates modified fields.
...
u.Email = "test@emailaddr.tld"
if err := u.Save(); err != nil {
	log.Fatal(err)
}

```

#### Delete a Record
Not really much to say here. Like saving, but use `.Delete()` instead.
```
if err := u.Delete(); err != nil {
	log.Fatal(err)
}
```

### Chaining Query Methods
The methods `.Where()`, `.Limit()`, `.Offset()`, `.Ascending()`, `.Descending()` all return the model, so you can chain them.

Order doesn't matter for these methods. For example put Ascending and Descending anywhere (as many times as you want), but the orders are based on which one you did first to last.

Methods `.Where()`, `.Limit()`, and `.Offset()` work on a "last call wins" basis. It will overwrite any prior calls. They are only meant for one use.

Further examples of these uses are below:

```
u := Users{}.New()
u.Descending("CoolPoints").Ascending("Username").Each(func(err error) bool {
	fmt.Println(u.Username, "-", u.CoolPoints)
	return true
})
```

```
u.Where("cool_points > 100").Descending("CoolPoints").Ascending("Username").Each(func(err error) bool {
	// Note, u.Save() and u.Delete() can run here
	fmt.Println(u.Username, "-", u.CoolPoints)
	//u.Username = "Bob"
	//u.Email = "nobody@vmons.com"
	//u.Save()
	return true
})

```

```
x := MyModel{}.New()
x.Where().Ascending("some_column_name").Ascending("some_column_name").Descending("some_column_name")
```

```
x := MyModel{}.New()
x.Where().Limit(int).Offest(int)
```

```
x := MyModel{}.New()
x.Offest(int).Ascending("some_column_name").Descending("some_column_name").Where().Limit(int)
```