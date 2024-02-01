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

func (sm *StateManager) AddState(stateName string, state State) {
	sm.Mutex.Lock()
	sm.states[stateName] = state
	sm.Mutex.Unlock()
}

func (sm *StateManager) SetState(state State) {
	sm.Mutex.Lock()
	if sm.currentState != nil {
		sm.currentState.Exit()
	}
	sm.currentState = state
	sm.Mutex.Unlock()
	sm.currentState.Enter()
}

func (sm *StateManager) GetState(stateName string) (State, error) {
	sm.Mutex.RLock()
	defer sm.Mutex.RUnlock()
	state, ok := sm.states[stateName]
	if !ok {
		return nil, fmt.Errorf("state with name \"%s\" does not exist in map, check spelling and use AddState() if necessary", stateName)
	}
	return state, nil
}
