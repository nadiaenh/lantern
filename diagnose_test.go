package main

import "testing"

func TestFirstLine(t *testing.T) {
	cases := map[string]string{
		"pong from host\n":                        "pong from host",
		"Warning: version skew\npong from host\n": "pong from host",
		"\n\nWarning: skew\n\npong\n":             "pong",
		"":                                        "",
		"Warning: only warnings\nWarning: still":  "Warning: only warnings\nWarning: still",
	}
	for in, want := range cases {
		if got := firstLine([]byte(in)); got != want {
			t.Errorf("firstLine(%q) = %q, want %q", in, got, want)
		}
	}
}
