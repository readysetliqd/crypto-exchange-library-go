// Package statemanager provides a basic framework for managing states in a
// concurrent-safe manner intended for use with the crypto exchange APIs throughout
// this repo.
//
// This file, defaults.go, provides default implementations of the State and
// Event interfaces.
package statemanager

import (
	"context"
)

// DefaultState is a placeholder implementation of the State interface. It can
// be embedded in user-defined states to ensure they satisfy the State interface.
type DefaultState struct{}

// Enter is called when the state is entered. This is a placeholder implementation.
func (s *DefaultState) Enter(prevState State) {
	// Do nothing. Placeholder implementation
	// fmt.Println("Entering state...")
}

// Exit is called when the state is exited. This is a placeholder implementation.
func (s *DefaultState) Exit(nextState State) {
	// Do nothing. Placeholder implementation
	// fmt.Println("Exiting state...")
}

// Update is called to update the state. This is a placeholder implementation.
func (s *DefaultState) Update(ctx context.Context) {
	// Do nothing. Placeholder implementation
	// fmt.Println("Updating state...")
}

// HandleEvent is called to handle an event. This is a placeholder implementation.
func (s *DefaultState) HandleEvent(ctx context.Context, event Event, responChan chan interface{}) error {
	// Do nothing. Placeholder implementation
	// fmt.Println("Handling event...")
	return nil
}

// DefaultEvent is a placeholder implementation of the Event interface. It can
// be embedded in user-defined events to ensure they satisfy the Event interface.
type DefaultEvent struct{}

// Process is called to process the event. This is a placeholder implementation.
func (e *DefaultEvent) Process(ctx context.Context) error {
	// Do nothing. Placeholder implementation
	// fmt.Println("Processing event...")
	return nil
}
