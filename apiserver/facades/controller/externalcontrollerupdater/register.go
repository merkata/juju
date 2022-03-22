// Copyright 2022 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package externalcontrollerupdater

import (
	"reflect"

	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/state"
)

// Registry describes the API facades exposed by some API server.
type Registry interface {
	// MustRegister adds a single named facade at a given version to the
	// registry.
	// Factory will be called when someone wants to instantiate an object of
	// this facade, and facadeType defines the concrete type that the returned
	// object will be.
	// The Type information is used to define what methods will be exported in
	// the API, and it must exactly match the actual object returned by the
	// factory.
	MustRegister(string, int, facade.Factory, reflect.Type)
}

// Register is called to expose a package of facades onto a given registry.
func Register(registry Registry) {
	registry.MustRegister("ExternalControllerUpdater", 1, func(ctx facade.Context) (facade.Facade, error) {
		return newStateAPI(ctx)
	}, reflect.TypeOf((*ExternalControllerUpdaterAPI)(nil)))
}

// newStateAPI creates a new server-side CrossModelRelationsAPI API facade
// backed by global state.
func newStateAPI(ctx facade.Context) (*ExternalControllerUpdaterAPI, error) {
	return NewAPI(
		ctx.Auth(),
		ctx.Resources(),
		state.NewExternalControllers(ctx.State()),
	)
}
