// TODO fix package and file level comments
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
package statemanager

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// #region data structures

type SMSystem struct {
	stateManagers map[int32]*StateManager
	mutex         sync.RWMutex
}

type StateManager struct {
	states       map[string]State
	currentState State
	prevState    State
	eventChan    chan Event
	responseChan chan interface{}
	ctx          context.Context
	cancel       context.CancelFunc
	mutex        sync.RWMutex
}

type State interface {
	Enter(prevState State)
	Exit(nextState State)
	Update(ctx context.Context)
	HandleEvent(ctx context.Context, event Event, responseChan chan interface{}) error
}

type Event interface {
	Process(ctx context.Context) error
}

// #endregion

// #region Instantiation and settup of state mangagement/managers/states

// TODO write docstrings
func StartStateManagement() *SMSystem {
	sms := &SMSystem{
		stateManagers: make(map[int32]*StateManager),
	}
	return sms
}

// TODO write docstrings
func (sms *SMSystem) NewStateManager(instanceID int32) *StateManager {
	ctx, cancel := context.WithCancel(context.Background())
	stateManager := &StateManager{
		currentState: nil,
		prevState:    nil,
		states:       make(map[string]State),
		eventChan:    make(chan Event),
		responseChan: make(chan interface{}),
		ctx:          ctx,
		cancel:       cancel,
	}
	sms.mutex.Lock()
	sms.stateManagers[instanceID] = stateManager
	sms.mutex.Unlock()
	return stateManager
}

// TODO write docstrings
func (sm *StateManager) AddState(stateName string, state State) {
	sm.mutex.Lock()
	sm.states[stateName] = state
	sm.mutex.Unlock()
}

// #endregion

// #region State getter methods

// TODO write docstrings
func (sm *StateManager) GetState(stateName string) (State, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	state, ok := sm.states[stateName]
	if !ok {
		return nil, fmt.Errorf("state with name \"%s\" does not exist in map, check spelling and use AddState() if necessary", stateName)
	}
	return state, nil
}

func (sm *StateManager) CurrentState() State {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.currentState
}

func (sm *StateManager) PreviousState() State {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.prevState
}

// #endregion

// #region State operation methods

// TODO write docstrings
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

// TODO write docstrings
func (sm *StateManager) Run() {
	for {
		select {
		case <-sm.ctx.Done():
			fmt.Println("state manager stopped |", sm.ctx.Err())
			return
		case event := <-sm.eventChan:
			err := sm.currentState.HandleEvent(sm.ctx, event, sm.responseChan)
			if err != nil {
				fmt.Println("error handling event: ", err)
			}
		default:
			sm.currentState.Update(sm.ctx)
		}
	}
}

func (sm *StateManager) SendEvent(event Event) {
	sm.eventChan <- event // send the event to the channel
}

func (sm *StateManager) ReceiveResponse() interface{} {
	return <-sm.responseChan // receive the response from the channel
}

// #endregion

// #region Context methods

func (sm *StateManager) Cancel() {
	sm.cancel()
}

func (sm *StateManager) WithValue(key, val interface{}) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.ctx = context.WithValue(sm.ctx, key, val)
}

func (sm *StateManager) WithDeadline(deadline time.Time) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.ctx, sm.cancel = context.WithDeadline(sm.ctx, deadline)
}

func (sm *StateManager) WithTimeout(timeout time.Duration) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.ctx, sm.cancel = context.WithTimeout(sm.ctx, timeout)
}

// #endregion
