// Package krakenspot is a comprehensive toolkit for interfacing with the Kraken
// Spot Exchange API. It enables WebSocket and REST API interactions, including
// subscription to both public and private channels. The package provides a
// client for initiating these interactions and a state manager for handling
// them.
//
// The statemanager.go file specifically contains the implementation of the
// state manager, which is used to manage the state of the KrakenClient. It
// includes the declaration of the State and Event interfaces, their default
// implementations, and the methods for adding, setting, and getting states.
// This file plays a crucial role in managing the state of interactions with
// the Kraken Spot Exchange API.
package krakenspot

import (
	"context"
	"fmt"
)

type State interface {
	Enter()
	Exit()
	Update(ctx context.Context)
	HandleEvent(event Event)
}

type DefaultState struct{}

func (s *DefaultState) Enter() {
	// Do nothing. Placeholder implementation
	fmt.Println("Entering state...")
}

func (s *DefaultState) Exit() {
	// Do nothing. Placeholder implementation
	fmt.Println("Exiting state...")
}

func (s *DefaultState) Update(ctx context.Context) {
	// Do nothing. Placeholder implementation
	fmt.Println("Updating state...")
}

func (s *DefaultState) HandleEvent(event Event) {
	// Do nothing. Placeholder implementation
	fmt.Println("Handling event...")
}

type Event interface {
	Process() error
}

type DefaultEvent struct{}

func (e *DefaultEvent) Process() error {
	// Do nothing. Placeholder implementation
	fmt.Println("Processing event...")
	return nil
}

// TODO write docstrings
func (kc *KrakenClient) StartStateManager(currentState ...State) error {
	var state State = nil
	if len(currentState) > 0 {
		if len(currentState) > 1 {
			return fmt.Errorf("too many args, expected 0 or 1")
		}
		state = kc.currentState
	}
	kc.StateManager = &StateManager{
		currentState: state,
		states:       make(map[string]State),
	}
	return nil
}

// TODO write docstrings
func (sm *StateManager) AddState(stateName string, state State) {
	sm.Mutex.Lock()
	sm.states[stateName] = state
	sm.Mutex.Unlock()
}

// TODO write docstrings
func (sm *StateManager) SetState(state State) {
	sm.Mutex.Lock()
	if sm.currentState != nil {
		sm.currentState.Exit()
	}
	sm.currentState = state
	sm.Mutex.Unlock()
	sm.currentState.Enter()
}

// TODO write docstrings
func (sm *StateManager) GetState(stateName string) (State, error) {
	sm.Mutex.RLock()
	defer sm.Mutex.RUnlock()
	state, ok := sm.states[stateName]
	if !ok {
		return nil, fmt.Errorf("state with name \"%s\" does not exist in map, check spelling and use AddState() if necessary", stateName)
	}
	return state, nil
}
