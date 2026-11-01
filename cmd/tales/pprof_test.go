package main

import "testing"

func TestEnvTruthy(t *testing.T) {
	cases := map[string]bool{
		"true":  true,
		"TRUE":  true,
		"True":  true,
		"1":     true,
		"yes":   true,
		"on":    true,
		" true ": true,
		"":      false,
		"0":     false,
		"false": false,
		"no":    false,
		"off":   false,
		"maybe": false,
	}
	for in, want := range cases {
		if got := envTruthy(in); got != want {
			t.Fatalf("envTruthy(%q) = %v, want %v", in, got, want)
		}
	}
}
