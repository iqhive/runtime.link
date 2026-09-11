package petstore

import (
	"reflect"
	"testing"

	"runtime.link/api"
)

func TestParameterNames(t *testing.T) {
	s := api.StructureOf(API{})
	want := map[string][]string{
		"UploadImageForPet": {"petId", "additionalMetadata", "file"},
		"AddPet":            {""},
		"SetPet":            {""},
		"FindByStatus":      {""},
		"FindByTags":        {""},
		"GetPet":            {""},
		"ModPet":            {""},
		"DeletePet":         {""},
	}
	got := map[string][]string{}
	for fn := range s.Iter() {
		got[fn.Name] = fn.Args
	}
	for name, args := range want {
		if !reflect.DeepEqual(got[name], args) {
			t.Errorf("%s Args = %q, want %q", name, got[name], args)
		}
	}
	upload := mustPetFn(t, s, "UploadImageForPet")
	if !upload.Named() {
		t.Error("UploadImageForPet should be Named")
	}
	if upload.InName(0) != "petId" || upload.InName(2) != "file" {
		t.Errorf("InName = %q %q", upload.InName(0), upload.InName(2))
	}
	if upload.Outs != nil {
		t.Errorf("error-only UploadImageForPet Outs=%q", upload.Outs)
	}
	add := mustPetFn(t, s, "AddPet")
	if add.Named() {
		t.Error("AddPet has an unnamed argument")
	}
	find := mustPetFn(t, s, "FindByStatus")
	if !reflect.DeepEqual(find.Outs, []string{""}) {
		t.Errorf("unnamed slice result Outs=%q", find.Outs)
	}
}

func mustPetFn(t *testing.T, s api.Structure, name string) api.Function {
	t.Helper()
	for fn := range s.Iter() {
		if fn.Name == name {
			return fn
		}
	}
	t.Fatalf("%s not found", name)
	return api.Function{}
}
