package xyz_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"runtime.link/xyz"
)

func TestExtern(t *testing.T) {
	type CustomData xyz.Extern[CustomData, any]

	type CustomDataType xyz.Extend[CustomData, any, struct {
		String xyz.Case[CustomData, string] `json:",string"`
	}]
	var CustomDataTypes = xyz.AccessorFor(CustomDataType.Values)

	var val CustomData = CustomDataTypes.String.As("hello")

	if val.String() != "hello" {
		t.Fatal("unexpected value")
	}

	pair, err := val.MarshalPair()
	if err != nil {
		t.Fatal(err)
	}
	k, v := pair.Split()
	if k != "String" {
		t.Fatal("unexpected value")
	}
	if fmt.Sprint(v) != "hello" {
		t.Fatal("unexpected value")
	}

	var data CustomData
	if err := json.Unmarshal([]byte(`"world"`), &data); err != nil {
		t.Fatal(err)
	}
	if data.String() != "world" {
		t.Fatal("unexpected value")
	}

	type CustomEnum xyz.Extern[CustomEnum, any]

	type Enum1 xyz.Extend[CustomEnum, any, struct {
		One xyz.Case[CustomEnum, int]
		Two xyz.Case[CustomEnum, int]
	}]
	var Enum1s = xyz.AccessorFor(Enum1.Values)
	type Enum2 xyz.Extend[CustomEnum, any, struct {
		Three xyz.Case[CustomEnum, int]
		Four  xyz.Case[CustomEnum, int]
	}]
	var Enum2s = xyz.AccessorFor(Enum2.Values)

	var enum CustomEnum = Enum1s.One.As(1)
	if xyz.ValueOf(enum).String() != "One" {
		t.Fatal("unexpected value")
	}
	var enum2 CustomEnum = Enum2s.Four.As(4)
	if enum2.String() != "4" {
		t.Fatal("unexpected value")
	}
}

func TestExtend(t *testing.T) {
	type ExternalID xyz.Extern[ExternalID, string]

	var OurExternalID = xyz.AccessorFor(xyz.Extend[ExternalID, string, struct {
		Value xyz.Case[ExternalID, string] `txt:"%v" json:"data?type=client.AccountID"`
	}].Values)

	var id ExternalID = OurExternalID.Value.New("hello")

	if id.String() != "hello" {
		t.Fatal("unexpected value")
	}
}
