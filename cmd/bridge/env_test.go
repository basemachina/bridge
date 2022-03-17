package main

import (
	"os"
	"testing"
)

func TestReadFromEnv(t *testing.T) {
	reset := setenvs(t, map[string]string{
		"PORT": "10000",
	})
	t.Cleanup(reset)

	env, err := ReadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	const wantPort = "10000"
	if got := env.Port; got != wantPort {
		t.Fatalf("got %v, want %v", got, wantPort)
	}
}

func setenv(t *testing.T, k, v string) func() {
	t.Helper()

	prev := os.Getenv(k)
	if err := os.Setenv(k, v); err != nil {
		t.Fatal(err)
	}
	return func() {
		if prev == "" {
			os.Unsetenv(k)
		} else {
			if err := os.Setenv(k, prev); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func setenvs(t *testing.T, kv map[string]string) func() {
	t.Helper()

	resetFuncs := make([]func(), 0, len(kv))
	for k, v := range kv {
		resetFunc := setenv(t, k, v)
		resetFuncs = append(resetFuncs, resetFunc)
	}
	return func() {
		for _, resetFunc := range resetFuncs {
			resetFunc()
		}
	}
}
