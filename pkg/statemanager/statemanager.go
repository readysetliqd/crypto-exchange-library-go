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
	"sync/atomic"
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
	isRunning    atomic.Bool
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

// InitialState embeds DefaultState with the only different that HandleEvent is
// also NoOp (no longer calls Process on the event). NewStateManager currentState
// and prevState fields are initialized as InitialState type.
type InitialState struct {
	DefaultState
}

func (s *InitialState) HandleEvent(ctx context.Context, event Event, responseChan chan interface{}) error {
	return nil
}

// NewStateManager creates a new StateManager for a given instance and adds it
// to the SMSystem. Arg passed to 'instanceID' must be unique. Accepts none or
// any functional options args passed to 'options'.
//
// Note: Returns existing state manager if duplicate IDs are passed and prints
// an error message to sms.errorLogger. Does not throw an error and does not
// overwrite with a new StateManager instance without first deleting the
// existing one with DeleteStateManager.
//
// # Functional Options:
//
//	// WithInitialState sets the currentState field of a StateManager to the given state. Defaults to InitialState type if not passed. Prints error message if nil state is passed and returns effectively leaving currentState as default.
//	func WithInitialState(initialState State) NewStateManagerOption
//
//	// WithoutRun prevents the StateManager from calling the Run method on startup. Defaults to calling Run on startup.
//	func WithoutRun() NewStateManagerOption
//
//	// WithAddState adds the state to the states map on initialization. Identical functionality as StateManager.AddState
//	func WithAddState(stateName string, state State) NewStateManagerOption {
func (sms *SMSystem) NewStateManager(instanceID int32, options ...NewStateManagerOption) *StateManager {
	sms.mutex.Lock()
	defer sms.mutex.Unlock()

	if _, ok := sms.stateManagers[instanceID]; ok {
		sms.errorLogger.Println(ErrInstanceIDExists.Error())
		return sms.stateManagers[instanceID]
	}

	// Instance with default values
	ctx, cancel := context.WithCancel(context.Background())
	stateManager := &StateManager{
		isRunning:    atomic.Bool{},
		currentState: &InitialState{},
		prevState:    &InitialState{},
		states:       make(map[string]State),
		eventChan:    make(chan Event, 3),
		responseChan: make(chan interface{}),
		errorLogger:  sms.errorLogger,
		ctx:          ctx,
		cancel:       cancel,
	}
	stateManager.isRunning.Store(true)
	sms.stateManagers[instanceID] = stateManager

	// Apply functional options
	for _, opt := range options {
		opt(stateManager)
	}

	if stateManager.isRunning.Load() { // can be set by WithoutRun functional option
		go stateManager.Run()
	}
	return stateManager
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
	sms.stateManagers[instanceID].Pause()
	time.Sleep(time.Millisecond * 50)
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

// SetState sets the current state of the StateManager to the given state and
// calls the appropriate state.Exit and state.Enter methods when state manager
// is running.
func (sm *StateManager) SetState(state State) {
	sm.mutex.Lock()
	sm.prevState = sm.currentState
	sm.mutex.Unlock()
	if sm.isRunning.Load() {
		sm.currentState.Exit(state)
	}
	sm.mutex.Lock()
	sm.currentState = state
	sm.mutex.Unlock()
	if sm.isRunning.Load() {
		sm.currentState.Enter(sm.prevState)
	}
}

// ForceState sets currentState to the arg passed to 'state' whilst skipping
// calling the appropriate Enter and Exit methods regardless if statemanager is
// running or not. Normal usage is to use SetState instead.
func (sm *StateManager) ForceState(state State) {
	sm.mutex.Lock()
	sm.prevState = sm.currentState
	sm.currentState = state
	sm.mutex.Unlock()
}

// Run starts the main loop of the StateManager, which handles events and
// updates the current state.
func (sm *StateManager) Run() {
	sm.isRunning.Store(true)
	for {
		select {
		case <-sm.ctx.Done():
			sm.isRunning.Store(false)
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

// IsRunning returns true if statemanager is running.
func (sm *StateManager) IsRunning() bool {
	return sm.isRunning.Load()
}

// Pause cancels the context of the StateManager, effectively stopping it. This
// skips any state.Exit logic and also prevents future Enter or Exit methods being
// called when setting state with SetState. It also returns from the Run goroutine
// effectively stopping listening for events, stopping HandleEvent method being
// called, and stopping Update methods being called continuously. It maintains
// currentState and prevState, use Reset instead if needed to force both back to
// InitialState. Resume statemanager operation with Run
func (sm *StateManager) Pause() {
	sm.cancel()
}

// Reset cancels the context of the StateManager, effectively stopping it, and
// assigns currentState and prevState fields back to InitialState while skipping
// state.Exit logic for the currently running state. If currentState and prevState
// need to be maintained, use Pause instead. Start statemanager again with Run.
func (sm *StateManager) Reset() {
	sm.cancel()
	time.Sleep(time.Millisecond * 50) // wait for goroutine to return
	sm.mutex.Lock()
	sm.currentState = &InitialState{}
	sm.prevState = &InitialState{}
	sm.mutex.Unlock()
}

// SendEvent sends an event to the StateManager's event channel when the
// StateManager is running.
func (sm *StateManager) SendEvent(event Event) {
	if sm.isRunning.Load() {
		sm.eventChan <- event
	}
}

// ReceiveResponse receives a response from the StateManager's response channel
// and returns it and a bool to signal a response was received.
//
// # Example Usage:
//
//	// Start a go routine that attempts reads from the response channel every second
//	go func() {
//		for {
//			// implement your own logic to break from this loop
//			resp, ok := sm.ReceiveResponse()
//			if ok {
//				log.Println(resp)
//			} else {
//				time.Sleep(time.Second)
//			}
//		}
//	}
func (sm *StateManager) ReceiveResponse() (interface{}, bool) {
	select {
	case resp := <-sm.responseChan:
		return resp, true
	default:
		return nil, false
	}
}

// #endregion

// #region Context methods

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
