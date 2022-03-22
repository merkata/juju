// Copyright 2022 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package migrationmaster

import (
	"reflect"

	"github.com/juju/errors"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/migration"
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
	registry.MustRegister("MigrationMaster", 1, func(ctx facade.Context) (facade.Facade, error) {
		return newMigrationMasterFacadeV1(ctx)
	}, reflect.TypeOf((*APIV1)(nil)))
	registry.MustRegister("MigrationMaster", 2, func(ctx facade.Context) (facade.Facade, error) {
		return newMigrationMasterFacadeV2(ctx)
	}, reflect.TypeOf((*APIV2)(nil)))
	registry.MustRegister("MigrationMaster", 3, func(ctx facade.Context) (facade.Facade, error) {
		return newMigrationMasterFacade(ctx) // Adds MinionReportTimeout.
	}, reflect.TypeOf((*API)(nil)))
}

// newMigrationMasterFacadeV1 exists to provide the required signature for API
// registration, converting st to backend.
func newMigrationMasterFacadeV1(ctx facade.Context) (*APIV1, error) {
	v2, err := newMigrationMasterFacadeV2(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &APIV1{v2}, nil
}

// newMigrationMasterFacadeV2 exists to provide the required signature for API
// registration, converting st to backend.
func newMigrationMasterFacadeV2(ctx facade.Context) (*APIV2, error) {
	v3, err := newMigrationMasterFacade(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &APIV2{v3}, nil
}

// newMigrationMasterFacade exists to provide the required signature for API
// registration, converting st to backend.
func newMigrationMasterFacade(ctx facade.Context) (*API, error) {
	controllerState := ctx.StatePool().SystemState()
	precheckBackend, err := migration.PrecheckShim(ctx.State(), controllerState)
	if err != nil {
		return nil, errors.Annotate(err, "creating precheck backend")
	}
	return NewAPI(
		newBacked(ctx.State()),
		precheckBackend,
		migration.PoolShim(ctx.StatePool()),
		ctx.Resources(),
		ctx.Auth(),
		ctx.Presence(),
	)
}
