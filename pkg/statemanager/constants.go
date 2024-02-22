package statemanager

import "errors"

var ErrInstanceIDExists = errors.New("error creating new StateManager; ID already exists")
