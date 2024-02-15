package statemanager

import (
	"context"
	"fmt"
)

type DefaultState struct{}

func (s *DefaultState) Enter(prevState State) {
	// Do nothing. Placeholder implementation
	fmt.Println("Entering state...")
}

func (s *DefaultState) Exit(nextState State) {
	// Do nothing. Placeholder implementation
	fmt.Println("Exiting state...")
}

func (s *DefaultState) Update(ctx context.Context) {
	// Do nothing. Placeholder implementation
	fmt.Println("Updating state...")
}

func (s *DefaultState) HandleEvent(ctx context.Context, event Event, responChan chan interface{}) error {
	// Do nothing. Placeholder implementation
	fmt.Println("Handling event...")
	return nil
}

type DefaultEvent struct{}

func (e *DefaultEvent) Process() error {
	// Do nothing. Placeholder implementation
	fmt.Println("Processing event...")
	return nil
}
