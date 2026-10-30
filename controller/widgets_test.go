package controller

import (
	"testing"

	"github.com/glgaspar/front_desk/features/apps"
)

func TestInDays(t *testing.T) {
	cases := map[float64]string{-1: "already expired", 0.5: "today", 1.5: "tomorrow", 10.2: "in 10 days"}
	for days, want := range cases {
		if got := inDays(days); got != want {
			t.Errorf("%v: got %q want %q", days, got, want)
		}
	}
}

func TestBroken(t *testing.T) {
	app := apps.App{}
	cases := []struct {
		status string
		code   int
		want   bool
	}{
		{"running", 0, false},
		{"exited", 0, false},
		{"exited", 1, true},
		{"restarting", 0, true},
		{"dead", 0, true},
	}
	for _, c := range cases {
		app.State.Status = c.status
		app.State.ExitCode = c.code
		if app.Broken() != c.want {
			t.Errorf("%s %d: got %v", c.status, c.code, !c.want)
		}
	}
}
