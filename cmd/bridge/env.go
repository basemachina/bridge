package main

import (
	"fmt"

	"github.com/basemachina/bridge"
	"github.com/kelseyhightower/envconfig"
)

// ReadFromEnv reads configuration from environmental variables
// defined by Env struct.
func ReadFromEnv() (*bridge.Env, error) {
	var env bridge.Env
	if err := envconfig.Process("", &env); err != nil {
		return nil, fmt.Errorf("failed to process envconfig: %w", err)
	}
	return &env, nil
}
