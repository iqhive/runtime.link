package xyz

/*
Extern is a special empty tagged type that can have additional cases added to it
by external (dependent) packages.

This enables the creation of foreign key or metadata values, where the range of
permissable types is defined by the users of an API.

	package system

	type ExternalID xyz.Extern[ExternalID, any]

	package client

	var SystemExternalID = xyz.AccessorFor(xyz.Extend[system.ExternalID, any, struct {
		Make xyz.Case[system.ExternalID, string] `json:"data?type=client.AccountID"`
	}].Values)

SystemExternalID.Make.As("1234") is then a system.ExternalID that is written as
{"data":"1234","type":"client.AccountID"}. An Extern with string storage holds
the text of its cases instead, so each of them needs a txt tag and none is
written by its json tag.
*/
type Extern[T any, Storage any] struct {
	taggedMethods[Storage, struct{ _ [0]T }]
}

// Extend an [Extern] type with the given switch values.
type Extend[Extern isVariantWith[Storage], Storage any, Values any] struct {
	taggedMethods[Storage, Values]
}
