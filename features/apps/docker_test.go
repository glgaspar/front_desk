package apps

import (
	"testing"
)

func TestValidID(t *testing.T) {
	good := []string{"abc123", "front_desk_api", "9f1c2a7e0b3d", "my-app.1"}
	bad := []string{"", "abc;rm -rf /", "$(whoami)", "a b", "-rm", "../etc", "abc|ls"}

	for _, id := range good {
		if !ValidID(id) {
			t.Errorf("%q should be valid", id)
		}
	}
	for _, id := range bad {
		if ValidID(id) {
			t.Errorf("%q should not be valid", id)
		}
	}
}

func TestAppPath(t *testing.T) {
	t.Setenv("APPS_DIR", "/src/apps")

	ok := map[string]string{
		"/jellyfin": "/src/apps/jellyfin",
		"jellyfin":  "/src/apps/jellyfin",
		"/a/b":      "/src/apps/a/b",
		"../etc":    "/src/apps/etc",
		"/x/../y":   "/src/apps/y",
	}
	for dir, want := range ok {
		got, err := AppPath(dir)
		if err != nil {
			t.Errorf("%q returned error %v", dir, err)
			continue
		}
		if got != want {
			t.Errorf("%q got %q want %q", dir, got, want)
		}
	}

	for _, dir := range []string{"", " ", "/", "..", "/..", "/x/.."} {
		if _, err := AppPath(dir); err == nil {
			t.Errorf("%q should fail", dir)
		}
	}
}
