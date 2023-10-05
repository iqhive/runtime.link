package ref_test

import (
	"context"
	"fmt"
	"testing"

	"runtime.link/ref"
)

type ExternalID ref.Any
type CustomerID ref.For[API, Customer, ExternalID]

func (id CustomerID) Lookup(ctx context.Context) (Customer, error) {
	return id.API.LookupCustomer(ctx, id)
}

type API struct {
	CreateCustomer func(context.Context, Customer) error
	LookupCustomer func(context.Context, CustomerID) (Customer, error)
}

type Customer struct {
	ID CustomerID

	Name string
}

func (cus Customer) Create(ctx context.Context) error { return cus.ID.API.CreateCustomer(ctx, cus) }

func TestRef(t *testing.T) {
	var ctx = context.Background()
	var db = make(map[ExternalID]Customer)

	var api = new(API)
	api.CreateCustomer = func(ctx context.Context, cus Customer) error {
		db[cus.ID.Ref] = cus
		return nil
	}
	api.LookupCustomer = func(ctx context.Context, id CustomerID) (Customer, error) {
		return db[id.Ref], nil
	}

	type MyCustomerID int
	const (
		MapExternalID = ref.Our[ExternalID, MyCustomerID]("mycustomer_%v")
	)

	var id = ref.New[CustomerID](api, MapExternalID.New(1234))

	var cus = Customer{
		ID:   id,
		Name: "Alice",
	}
	if err := cus.Create(ctx); err != nil {
		t.Fatal(err)
	}

	alice, err := id.Lookup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if alice.Name != cus.Name {
		t.Fatalf("alice.Name = %q; want %q", alice.Name, cus.Name)
	}

	fmt.Println(MapExternalID.Get(alice.ID.Ref))
}
