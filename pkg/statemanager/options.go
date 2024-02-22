package statemanager

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

// WithoutRun prevents the StateManager from calling the Run method on startup.
// Defaults to calling Run on startup.
func WithoutRun() NewStateManagerOption {
	return func(sm *StateManager) {
		sm.isRunning.Store(false) // set the isRunning flag to true to skip the Run method
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
