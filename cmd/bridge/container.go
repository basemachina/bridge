package main

import (
	"net/http"

	"github.com/basemachina/bridge"
	"github.com/basemachina/bridge/internal/auth"
	"github.com/go-logr/logr"
)

type Container struct {
	HTTPServer  *http.Server
	FetchWorker *FetchWorker
	Logger      logr.Logger
}

func BridgeContainerProvider() (*Container, func(), error) {
	env, err := ReadFromEnv()
	if err != nil {
		return nil, nil, err
	}
	logger, cleanup, err := NewLogger(env)
	if err != nil {
		return nil, nil, err
	}
	fetchWorker, cleanup2, err := NewFetchWorker(env, logger)
	if err != nil {
		return nil, nil, err
	}
	httpHandlerConfig := &bridge.HTTPHandlerConfig{
		Logger:             logger,
		PublicKeyGetter:    fetchWorker,
		TenantID:           env.TenantID,
		RegisterUserObject: auth.User{},
	}
	handler := bridge.NewHTTPHandler(httpHandlerConfig)
	server, cleanup3, err := NewHTTPServer(env, handler)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	container := &Container{
		HTTPServer:  server,
		FetchWorker: fetchWorker,
		Logger:      logger,
	}
	return container, func() {
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}
