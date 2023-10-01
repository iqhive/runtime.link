package sodium

import (
	"context"

	"runtime.link/api"
	"runtime.link/xyz"
)

type API struct {
	api.Specification `
		for communicating with a SODIUM database host.`

	Socket func(context.Context) (Socket, error) `rest:"GET /" sock:"websocket"`
}

type Socket struct {
	api.Specification

	//sock.Close
	//sock.Reset

	Read func(context.Context) (Result, error) `txt:"SYNC" xyz:"0"
		waits for a result and returns it.`

	Search func(context.Context, Table, Query) error `txt:"SEARCH(table,query)" xyz:"1(1,2)"
		starts the execution of a search query.`
	Output func(context.Context, Table, Query, Stats) error `txt:"OUTPUT(table,query,stats)" xyz:"2(1,2,3)"
		starts the execution of an output query.`
	Delete func(context.Context, Table, Query) error `txt:"DELETE(table,query)" xyz:"3(1,2)"
		starts the execution of a delete query.`
	Insert func(context.Context, Table, []Value, bool, []Value) error `txt:"INSERT(table,index,flag,value)" xyz:"4(1,2,3,4)"
		starts the execution of an insert query.`
	Update func(context.Context, Table, Query, Patch) error `txt:"UPDATE(table,query,patch)" xyz:"3(1,2,3)"
		starts the execution of an update query.`
	Manage func(context.Context, Transaction) error `txt:"MANAGE" xyz:"6(1)"
		commits the current transaction and starts a new transaction
		with the specified transaction level. `
}

type Result struct {
	Number int `txt:"number" xyz:"0"
		is the Nth command from the socket 
		that this result corresponds to.`
	Closed bool `txt:"closed" xyz:"1"
		indicates that the there are no
		more future results for this number.`
	Values []Value `txt:"values" xyz:"2"
		are the current set of results.`
	Errors []Error `txt:"errors" xyz:"3"
		are any errors that occurred during
		the execution of the command.`
}

type Error xyz.Switch[any, struct {
	Internal Error `txt:"internal" xyz:"0"`
}]

var Errors = xyz.AccessorFor(Error.Values)
