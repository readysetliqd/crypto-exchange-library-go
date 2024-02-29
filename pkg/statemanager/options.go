package statemanager

import "time"

// StateManagerOption is a type for the option functions that modify a StateManager
type NewStateManagerOption func(*StateManager)

// WithInitialState sets the currentState field of a StateManager to the given
// state. Defaults to InitialState type if not passed. Prints error message if
// nil state is passed and returns effectively leaving currentState as default.
func WithInitialState(initialState State) NewStateManagerOption {
	return func(sm *StateManager) {
		if initialState == nil {
			sm.errorLogger.Println("cannot pass nil state to initialState")
			return
		}
		sm.currentState = initialState
	}
}

// WithRunTypeNoRun prevents the StateManager from calling the Run method on
// startup.
//
// Note: Default without this option is calling Run on startup. Mutually
// exclusive with other WithRunType<type> methods, only call one of these
// methods per NewStateManager.
func WithRunTypeNoRun() NewStateManagerOption {
	return func(sm *StateManager) {
		sm.runType = withoutRun
	}
}

// WithRunTypeNoUpdate calls RunWithoutUpdate method on startup.
//
// Note: Default without this option is calling Run on startup. Mutually
// exclusive with other WithRunType<type> methods, only call one of these
// methods per NewStateManager.
func WithRunTypeNoUpdate() NewStateManagerOption {
	return func(sm *StateManager) {
		sm.runType = withoutUpdate
	}
}

// WithRunTypeCustomUpdate calls Run(interval) method on startup where arg
// 'interval' is the time which the go routine sleeps between Update calls.
//
// Note: Default without this option is calling Run on startup. Mutually
// exclusive with other WithRunType<type> methods, only call one of these
// methods per NewStateManager.
func WithRunTypeCustomUpdate(interval time.Duration) NewStateManagerOption {
	return func(sm *StateManager) {
		sm.updateInterval = interval
		sm.runType = customUpdateInterval
	}
}

// WithRunTypeContinuousUpdate calls RunWithContinuous method on startup.
// Generally this is not recommended as it's compute heavy.
//
// Note: Default without this option is calling Run on startup. Mutually
// exclusive with other WithRunType<type> methods, only call one of these
// methods per NewStateManager.
func WithRunTypeContinuousUpdate() NewStateManagerOption {
	return func(sm *StateManager) {
		sm.runType = continuousUpdate
	}
}

// WithAddState adds the state to the states map on initialization. Identical
// functionality as StateManager.AddState
func WithAddState(stateName string, state State) NewStateManagerOption {
	return func(sm *StateManager) {
		sm.mutex.Lock()
		sm.states[stateName] = state
		sm.mutex.Unlock()
	}
}
