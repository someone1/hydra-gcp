package main

import (
	"github.com/ory/hydra/config"
	gconfig "github.com/someone1/hydra-gcp/config"
)

var BackendConnector config.BackendConnector = &gconfig.DatastoreConnection{}
