package api_test

import (
	"runtime"
	"testing"

	"github.com/iqhive/runtime.link/api"
)

// host implements [api.Host] the way call.To does.
type host struct {
	api.Host
}

type embeddedHost struct {
	host `lib:"embedded"`
}

// libraryFor returns the tag an API with hosts for these three systems expects on this one.
func libraryFor(t *testing.T) string {
	t.Helper()
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		return "lib-" + runtime.GOOS
	}
	t.Skip("no host declared for", runtime.GOOS)
	return ""
}

func TestHostTagForThisGOOS(t *testing.T) {
	want := libraryFor(t)

	type ordered struct {
		linux   host `lib:"lib-linux"`
		darwin  host `lib:"lib-darwin"`
		windows host `lib:"lib-windows"`
	}
	if got := api.StructureOf(&ordered{}).Host.Get("lib"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	type reversed struct {
		windows host `lib:"lib-windows"`
		darwin  host `lib:"lib-darwin"`
		linux   host `lib:"lib-linux"`
	}
	if got := api.StructureOf(&reversed{}).Host.Get("lib"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	type overSpecification struct {
		linux             host `lib:"lib-linux"`
		darwin            host `lib:"lib-darwin"`
		windows           host `lib:"lib-windows"`
		api.Specification `lib:"default"`
	}
	if got := api.StructureOf(&overSpecification{}).Host.Get("lib"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	type overGeneric struct {
		api.Specification

		linux   host `lib:"lib-linux"`
		darwin  host `lib:"lib-darwin"`
		windows host `lib:"lib-windows"`
		Fixed   host `lib:"fixed"`
	}
	if got := api.StructureOf(&overGeneric{}).Host.Get("lib"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHostTagFallsBackWithoutThisGOOS(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("plan9 is the system this test has no host for")
	}
	type overSpecification struct {
		api.Specification `lib:"default"`

		plan9 host `lib:"plan9-only"`
	}
	if got := api.StructureOf(&overSpecification{}).Host.Get("lib"); got != "default" {
		t.Errorf("a host for another system beat the specification: %q", got)
	}

	type overGeneric struct {
		Fixed host `lib:"fixed"`
		plan9 host `lib:"plan9-only"`
	}
	if got := api.StructureOf(&overGeneric{}).Host.Get("lib"); got != "fixed" {
		t.Errorf("a host for another system beat a host for all of them: %q", got)
	}

	type none struct {
		plan9 host `lib:"plan9-only"`
	}
	if got := api.StructureOf(&none{}).Host; got != "" {
		t.Errorf("a host for another system was used: %q", got)
	}
}

func TestEmptyHostTagFallsBack(t *testing.T) {
	libraryFor(t)
	type empty struct {
		api.Specification `lib:"default"`

		linux   host
		darwin  host
		windows host
	}
	if got := api.StructureOf(&empty{}).Host.Get("lib"); got != "default" {
		t.Errorf("got %q, want the specification's tag", got)
	}
}

func TestEmbeddedHostFindsItsTag(t *testing.T) {
	type embedded struct {
		embeddedHost
	}
	s := api.StructureOf(&embedded{})
	if got := s.Host.Get("lib"); got != "embedded" {
		t.Errorf("got %q", got)
	}
	if _, ok := s.Namespace["embeddedHost"]; !ok {
		t.Errorf("the namespace was filed under a different field: %v", s.Namespace)
	}

	// An embedded host with no tag anywhere ends at the api.Host interface.
	type bare struct {
		host
	}
	if got := api.StructureOf(&bare{}).Host; got != "" {
		t.Errorf("got %q", got)
	}
}
