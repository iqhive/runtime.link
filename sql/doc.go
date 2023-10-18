/*
Package sql defines a data map type with support for type-safe structured queries.

# Designing a Schema

Domain objects should typically be split into comparable index and value pairs.

	// Domain type.
	type Customer struct {
		ID int64
		Age uint
		Name string
	}

	// Internal Database Index Type.
	type dbID int64

	// Internal Database Value Type.
	type dbCustomer struct {
		Name string
		Age uint
	}

	// Database representation.
	var customers sql.Map[dbID, dbCustomer]

Insert a record into the table, overwriting it if the ID already exists.

	err := customers.Insert(ctx, id, &dbCustomer{Name: "Bob", Age: 23})

Lookup a record from the table.

	customer, exists, err := customers.Lookup(ctx, id)

Delete the record from the table.

	err := customers.Delete(ctx, id)

# Search

It is possible to search for records in the table and iterate through
the results. For example, to search for customers with the age "22":

	query := func(index *dbID, value *dbCustomer) sql.Query {
		return sql.Query{
			sql.Where(&value.Age).Equals(22),
		}
	}
	for result := range customers.SearchFunc(ctx, query) {
		id, customer, err := result()
		if err != nil {
			return xray.Error(err)
		}
		fmt.Println(customer.Name)
	}

# Query Builder

Queries can be constructed using a series of operations:

	sql.Where(&field).Equals(x)   // field == x
	sql.Where(&field).AtLeast(x)  // field >= x
	sql.Where(&field).AtMost(x)   // field <= x
	sql.Where(&field).MoreThan(x) // field > x
	sql.Where(&field).LessThan(x) // field < x

	sql.Match(&field).Contains(x)  // strings.Contains(field, x)
	sql.Match(&field).HasPrefix(x) // strings.HasPrefix(field, x)
	sql.Match(&field).HasSuffix(x) // strings.HasSuffix(field, x)

	sql.Order(&field).Increasing() // ORDER BY field ASC
	sql.Order(&field).Decreasing() // ORDER BY field DESC

	sql.Slice(from, upto) // data[from:upto]

	sql.Empty(&field) // reflect.ValueOf(field).IsZero()
	sql.Avoid(q)      // !q

	// switch
	sql.Cases(
		sql.Where(&field).Equals(x),
		sql.Where(&field).Equals(y),
		...
	)

# Conditional Writes

Each write operation supports conditionals, so that they can only be completed
when the provided checks match the existing record (which may be a zero value).

	ifBob := func(index *dbUUID, value *dbCustomer) sql.Query {
		return sql.Query{
			sql.Where(&index).Equals(1),
			sql.Where(&value.Name).Equals("Bob",),
		}
	}
	ok, err := customers.UpdateFunc(ctx, ifBob, func(value dbCustomer) sql.Patch {
		return sql.Patch{
			sql.Set(&value.Name, "Alice"),
		}
	}))

In the example above, the Customer with ID 1 will only have their name
updated to "Alice" if their name was "Bob". If the customer's names is
not Bob, ok will be false.

# Statistics

Statistics can be reported on a map in the database with [sql.Counter].

	var (
		count sql.Counter[uint]
		total sql.Counter[uint]
	)
	stats := func(index dbUUID, value dbCustomer) sql.Stats {
		return sql.Stats{
			count.Add(),
			total.Sum(&value.Age),
		}
	}
	err := customers.OutputFunc(ctx, stats, func(index *dbUUID, value *dbCustomer) sql.Query {
		return sql.Query{
			sql.Where(&value.Name).Equals("Bob"),
		}
	})
*/
package sql
