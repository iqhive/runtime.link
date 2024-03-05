package xyz

/*
Extern is a special empty switch type that can have additional cases added to it
by external (dependent) packages.

This enables the creation of foreign key or metadata values, where the range of
permissable types is defined by the users of an API.

	package system

	type ExternalID xyz.Extern[ExternalID, string]

	package client

	var SystemExternalID xyz.AccessorFor(xyz.Extend[system.ExternalID, string, struct {
		Make xyz.Case[api.ExternalID, string] `json:"data?type=client.AccountID"`
	}].Values)
*/
type Extern[T any, Storage any] struct {
	switchMethods[Storage, struct{ _ [0]T }]
}

// Extend an [Extern] type with the given switch values.
type Extend[Extern isVariantWith[Storage], Storage any, Values any] struct {
	switchMethods[Storage, Values]
}
