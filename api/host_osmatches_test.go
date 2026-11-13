package api

import "testing"

func TestOSMatchesFollowsDerivedGOOS(t *testing.T) {
	for _, c := range []struct {
		name, goos string
		want       bool
	}{
		{"linux", "linux", true},
		{"linux", "android", true},
		{"android", "linux", false},
		{"darwin", "ios", true},
		{"ios", "darwin", false},
		{"solaris", "illumos", true},
		{"illumos", "solaris", false},
		{"linux", "darwin", false},
		{"", "linux", false},
		{"", "", false},
	} {
		if got := osMatches(c.name, c.goos); got != c.want {
			t.Errorf("osMatches(%q, %q) = %v", c.name, c.goos, got)
		}
	}
}
