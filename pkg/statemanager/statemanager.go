// Package statemanager provides a basic framework for managing states in a
// concurrent-safe manner intended for use with the crypto exchange APIs throughout
// this repo.
//
// The file statemanager.go contains the implementation of this package. It
// includes the definition of states, events, and their handlers, as well as
// the management of state transitions.
package statemanager

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// #region data structures

// SMSystem is a system that manages multiple StateManagers.
type SMSystem struct {
	stateManagers map[int32]*StateManager
	errorLogger   *log.Logger
	mutex         sync.RWMutex
}

// StateManager manages the states and transitions of a single instance.
type StateManager struct {
	states       map[string]State
	currentState State
	prevState    State
	eventChan    chan Event
	responseChan chan interface{}
	errorLogger  *log.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	mutex        sync.RWMutex
}

// State represents a state in the state machine.
type State interface {
	Enter(prevState State)
	Exit(nextState State)
	Update(ctx context.Context)
	HandleEvent(ctx context.Context, event Event, responseChan chan interface{}) error
}

// Event represents an event that can trigger state transitions or actions.
type Event interface {
	Process(ctx context.Context) error
}

// #endregion

// #region Instantiation and setup of state mangagement/managers/states

// StartStateManagement initializes a new SMSystem and returns its pointer.
func StartStateManagement() *SMSystem {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	sms := &SMSystem{
		stateManagers: make(map[int32]*StateManager),
		errorLogger:   logger,
	}
	return sms
}

// SetErrorLogger creates a new custom error logger for SMSystem and its
// StateManagers. The logger logs to the provided 'output' io.Writer. This
// method also sets the created logger as the errorLogger for the SMSystem and
// all its existing StateManagers. It returns the newly created logger.
//
// Note: This method will overwrite the errorLogger for any StateManagers that
// are already created.
//
// # Example Usage:
//
//	// Open a file for logging
//	file, err := os.OpenFile("log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer file.Close()
//
//	// Create a new SMS
//	sms := StartStateManagement()
//	logger := sms.SetErrorLogger(file)
//
//	// Create new Kraken Client and set same file for its logger
//	kc := NewKrakenClient(apiKey, secretKey, 2)
//	kc.SetErrorLogger(file)
func (sms *SMSystem) SetErrorLogger(output io.Writer) *log.Logger {
	logger := log.New(output, "", log.LstdFlags)
	sms.errorLogger = logger
	if sms.stateManagers != nil {
		for _, sm := range sms.stateManagers {
			sm.errorLogger = logger
		}
	}
	return logger
}

// NewStateManager creates a new StateManager for a given instance and adds it
// to the SMSystem. Arg passed to 'instanceID' must be unique, returns an error
// if duplicate IDs are passed without first deleting it with DeleteStateManager.
func (sms *SMSystem) NewStateManager(instanceID int32) (*StateManager, error) {
	sms.mutex.Lock()
	defer sms.mutex.Unlock()

	if _, ok := sms.stateManagers[instanceID]; ok {
		return nil, fmt.Errorf("error creating state manager; state manager with instanceID %v already exists", instanceID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	stateManager := &StateManager{
		currentState: nil,
		prevState:    nil,
		states:       make(map[string]State),
		eventChan:    make(chan Event, 3),
		responseChan: make(chan interface{}),
		ctx:          ctx,
		cancel:       cancel,
		errorLogger:  sms.errorLogger,
	}
	sms.stateManagers[instanceID] = stateManager
	return stateManager, nil
}

// DeleteStateManager removes the StateManager associated with the given
// instanceID from the SMSystem. It returns an error if a StateManager with
// the given instanceID does not exist in the SMSystem.
func (sms *SMSystem) DeleteStateManager(instanceID int32) error {
	sms.mutex.Lock()
	defer sms.mutex.Unlock()

	if _, ok := sms.stateManagers[instanceID]; !ok {
		return fmt.Errorf("state manager with 'instanceID' %v does not exist", instanceID)
	}
	delete(sms.stateManagers, instanceID)
	return nil
}

// AddState adds a new state to the StateManager. Overwrites state if duplicate
// 'stateName' are entered.
func (sm *StateManager) AddState(stateName string, state State) {
	sm.mutex.Lock()
	sm.states[stateName] = state
	sm.mutex.Unlock()
}

// DeleteState deletes a state with 'stateName' from the StateManager. Returns
// an error if 'stateName' does not exist in StateManager.states map.
func (sm *StateManager) DeleteState(stateName string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if _, ok := sm.states[stateName]; !ok {
		return fmt.Errorf("error deleting state with stateName %s; does not exist", stateName)
	}
	delete(sm.states, stateName)
	return nil
}

// #endregion

// #region State getter methods

// GetState returns a state from the StateManager by its name 'stateName' or
// returns an error if it one hasn't been added by that name.
func (sm *StateManager) GetState(stateName string) (State, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	state, ok := sm.states[stateName]
	if !ok {
		return nil, fmt.Errorf("state with name \"%s\" does not exist in map, check spelling and use AddState() if necessary", stateName)
	}
	return state, nil
}

// CurrentState returns the current state of the StateManager.
func (sm *StateManager) CurrentState() State {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.currentState
}

// PreviousState returns the previous state of the StateManager.
func (sm *StateManager) PreviousState() State {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.prevState
}

// #endregion

// #region State operation methods

// SetState sets the current state of the StateManager to the given state.
func (sm *StateManager) SetState(state State) {
	sm.mutex.Lock()
	if sm.currentState != nil {
		sm.prevState = sm.currentState
		sm.currentState.Exit(state)
	}
	sm.currentState = state
	sm.mutex.Unlock()
	sm.currentState.Enter(sm.prevState)
}

// Run starts the main loop of the StateManager, which handles events and
// updates the current state.
func (sm *StateManager) Run() {
	for {
		select {
		case <-sm.ctx.Done():
			sm.errorLogger.Println("state manager stopped |", sm.ctx.Err())
			return
		case event := <-sm.eventChan:
			err := sm.currentState.HandleEvent(sm.ctx, event, sm.responseChan)
			if err != nil {
				sm.errorLogger.Println("error handling event: ", err)
			}
		default:
			sm.currentState.Update(sm.ctx)
		}
	}
}

// SendEvent sends an event to the StateManager's event channel.
func (sm *StateManager) SendEvent(event Event) {
	sm.eventChan <- event
}

// ReceiveResponse receives a response from the StateManager's response channel
// and returns it.
func (sm *StateManager) ReceiveResponse() interface{} {
	return <-sm.responseChan
}

// #endregion

// #region Context methods

// Cancel cancels the context of the StateManager, effectively stopping it.
func (sm *StateManager) Cancel() {
	sm.cancel()
}

// WithValue adds a value to the StateManager's context.
func (sm *StateManager) WithValue(key, val interface{}) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.ctx = context.WithValue(sm.ctx, key, val)
}

// WithDeadline sets a deadline on the StateManager's context.
func (sm *StateManager) WithDeadline(deadline time.Time) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.ctx, sm.cancel = context.WithDeadline(sm.ctx, deadline)
}

// WithTimeout sets a timeout on the StateManager's context.
func (sm *StateManager) WithTimeout(timeout time.Duration) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.ctx, sm.cancel = context.WithTimeout(sm.ctx, timeout)
}

// #endregion
