// Copyright 2022 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasunitprovisioner

import (
	"reflect"

	"github.com/juju/clock"
	"github.com/juju/errors"

	charmscommon "github.com/juju/juju/apiserver/common/charms"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/caas"
	"github.com/juju/juju/state"
	"github.com/juju/juju/state/stateenvirons"
	"github.com/juju/juju/storage/poolmanager"
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
	registry.MustRegister("CAASUnitProvisioner", 2, func(ctx facade.Context) (facade.Facade, error) {
		return newStateFacade(ctx)
	}, reflect.TypeOf((*Facade)(nil)))
}

// newStateFacade provides the signature required for facade registration.
func newStateFacade(ctx facade.Context) (*Facade, error) {
	authorizer := ctx.Auth()
	resources := ctx.Resources()
	sb, err := state.NewStorageBackend(ctx.State())
	if err != nil {
		return nil, errors.Trace(err)
	}
	db, err := state.NewDeviceBackend(ctx.State())
	if err != nil {
		return nil, errors.Trace(err)
	}

	model, err := ctx.State().Model()
	if err != nil {
		return nil, errors.Trace(err)
	}
	broker, err := stateenvirons.GetNewCAASBrokerFunc(caas.New)(model)
	if err != nil {
		return nil, errors.Annotate(err, "getting caas client")
	}
	registry := stateenvirons.NewStorageProviderRegistry(broker)
	pm := poolmanager.New(state.NewStateSettings(ctx.State()), registry)

	commonState := &charmscommon.StateShim{ctx.State()}
	charmInfoAPI, err := charmscommon.NewCharmInfoAPI(commonState, authorizer)
	if err != nil {
		return nil, errors.Trace(err)
	}
	appCharmInfoAPI, err := charmscommon.NewApplicationCharmInfoAPI(commonState, authorizer)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return NewFacade(
		resources,
		authorizer,
		stateShim{ctx.State()},
		sb,
		db,
		pm,
		registry,
		charmInfoAPI,
		appCharmInfoAPI,
		clock.WallClock,
	)
}
