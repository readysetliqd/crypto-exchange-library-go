package statemanager

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MockState is a mock implementation of the State interface for testing.
type MockState struct{}

func (s *MockState) Enter(prevState State)      {}
func (s *MockState) Exit(nextState State)       {}
func (s *MockState) Update(ctx context.Context) {}
func (s *MockState) HandleEvent(ctx context.Context, event Event, responseChan chan interface{}) error {
	return nil
}

// MockEvent is a mock implementation of the Event interface for testing.
type MockEvent struct{}

func (e *MockEvent) Process() {}

// MockDefaultState is a mock implementation embedding DefaultState for testing
type MockDefaultState struct {
	DefaultState
}

// MockDefaultEvent is a mock implementation embedding DefaultEvent for testing
type MockDefaultEvent struct {
	DefaultEvent
}

func TestStartStateManagement(t *testing.T) {
	sms := StartStateManagement()

	if sms == nil {
		t.Errorf("StartStateManagement(): %v; want: non-nil", sms)
	}

	if len(sms.stateManagers) != 0 {
		t.Errorf("StartStateManagement() len(sms.stateManagers): %v; want: map of length 0", len(sms.stateManagers))
	}
}

type MockState_NewStateManager struct {
	MockDefaultState
	entered      bool
	updateCalled bool
}

func (s *MockState_NewStateManager) Enter(prevState State) {
	s.entered = true
}

func (s *MockState_NewStateManager) Update(ctx context.Context) {
	s.updateCalled = true
}

func TestSMSystem_NewStateManager(t *testing.T) {
	sms := StartStateManagement()

	t.Run("normal valid use without options", func(t *testing.T) {
		// check state manager was initialized
		sm := sms.NewStateManager(1)
		if sm == nil {
			t.Fatalf("NewStateManager(): %v; want: non-nil", sms)
		}

		// check sm was added to the map at the correct key
		if sms.stateManagers[1] != sm {
			t.Errorf("NewStateManager() did not add state manager to the stateManagers map at the correct key")
		}

		// check fields initialized correctly
		currentState := sm.currentState
		switch ty := currentState.(type) {
		case *InitialState:
		default:
			t.Errorf("NewStateManager() currentState: %v; want: *InitialState", ty)
		}
		prevState := sm.prevState
		switch ty := prevState.(type) {
		case *InitialState:
		default:
			t.Errorf("NewStateManager() prevState: %v; want: *InitialState", ty)
		}
		if len(sm.states) != 0 {
			t.Errorf("NewStateManager() len(sm.states): %v; want: map of length 0", sm.states)
		}
		if sm.eventChan == nil {
			t.Errorf("NewStateManager() eventChan: %v; want: non-nil", sm.eventChan)
		}
		if sm.responseChan == nil {
			t.Errorf("NewStateManager() responseChan: %v; want: non-nil", sm.responseChan)
		}
		if sm.ctx == nil {
			t.Errorf("NewStateManager() ctx: %v; want: non-nil", sm.ctx)
		}
		if sm.cancel == nil {
			t.Errorf("NewStateManager() cancel: %v; want: non-nil", sm.cancel)
		}
		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})

	t.Run("duplicate instanceIDs", func(t *testing.T) {
		// check that NewStateManager returns an error when duplicate ID is used
		buf := new(bytes.Buffer)
		sms.SetErrorLogger(buf)
		sm1 := sms.NewStateManager(1)
		sm2 := sms.NewStateManager(1)

		// Check same state manager pointer was returned
		if sm1 != sm2 {
			t.Errorf("NewStateManager() did not return same state manager when duplicate ids passed")
		}

		// Check error message was written to errorLogger
		expected := true
		actual := strings.Contains(buf.String(), ErrInstanceIDExists.Error())
		if expected != actual {
			t.Errorf("NewStateManager() did not write error to buffer | got: %v; want: %v", actual, expected)
		}

		// Tear down
		sm1.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})

	t.Run("WithInitialState", func(t *testing.T) {
		state1 := &MockState_NewStateManager{}
		sm := sms.NewStateManager(1, WithInitialState(state1))
		expected := state1
		actual := sm.CurrentState()
		if expected != actual {
			t.Errorf("NewStateManager() did not set currentState correctly | got: %v; want: %v", actual, expected)
		}
		if state1.entered {
			t.Errorf("NewStateManager() called enter on state passed to initialState")
		}
		time.Sleep(time.Millisecond * 100)
		if !state1.updateCalled {
			t.Errorf("NewStateManager() not calling Update on initial state passed when running")
		}
		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})

	t.Run("WithRunTypeNoRun", func(t *testing.T) {
		sm := sms.NewStateManager(1, WithRunTypeNoRun())
		state1 := &MockState_NewStateManager{}
		sm.SetState(state1)
		time.Sleep(time.Millisecond * 20)
		if state1.entered {
			t.Errorf("NewStateManager() called enter when not running")
		}
		if state1.updateCalled {
			t.Errorf("NewStateManager() calling update when not running")
		}
		sm.Run()
		time.Sleep(time.Millisecond * 20)
		if state1.entered {
			t.Errorf("NewStateManager() called enter when state was set before running")
		}
		if !state1.updateCalled {
			t.Errorf("NewStateManager() did not call update when running")
		}
		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})

	t.Run("WithAddState multiple entries", func(t *testing.T) {
		state1 := &MockDefaultState{}
		state2 := &MockDefaultState{}
		state3 := &MockDefaultState{}
		sm := sms.NewStateManager(1, WithAddState("state1", state1), WithAddState("state2", state2), WithAddState("state3", state3))

		// Check states exist in the StateManager states map and return with GetState
		expected := state1
		actual, err := sm.GetState("state1")
		if err != nil {
			t.Errorf("NewStateManager() error getting state | got: %v; want: nil", err)
		}
		if expected != actual {
			t.Errorf("NewStateManager() returned incorrect state | got: %v; want: %v", actual, expected)
		}
		expected = state2
		actual, err = sm.GetState("state2")
		if err != nil {
			t.Errorf("NewStateManager() error getting state | got: %v; want: nil", err)
		}
		if expected != actual {
			t.Errorf("NewStateManager() returned incorrect state | got: %v; want: %v", actual, expected)
		}
		expected = state3
		actual, err = sm.GetState("state3")
		if err != nil {
			t.Errorf("NewStateManager() error getting state | got: %v; want: nil", err)
		}
		if expected != actual {
			t.Errorf("NewStateManager() returned incorrect state | got: %v; want: %v", actual, expected)
		}
		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})
}

func TestSMSystem_DeleteStateManager(t *testing.T) {
	sms := StartStateManagement()

	// Add a StateManager to the system.
	sms.NewStateManager(1)

	// Test deleting the StateManager.
	err := sms.DeleteStateManager(1)
	if err != nil {
		t.Errorf("DeleteStateManager() error: %v; want: nil", err)
	}

	// Check that the StateManager was removed.
	if _, ok := sms.stateManagers[1]; ok {
		t.Errorf("DeleteStateManager() did not remove state manager")
	}

	// Test deleting a StateManager that doesn't exist.
	err = sms.DeleteStateManager(2)
	if err == nil {
		t.Errorf("DeleteStateManager() error: nil; want: non-nil")
	}
}

func TestStateManager_AddState(t *testing.T) {
	sms := StartStateManagement()
	state := &MockState{}
	sm := sms.NewStateManager(1, WithRunTypeNoRun())
	sm.AddState("mock state", state)
	if sm.states["mock state"] != state {
		t.Errorf("AddState() state was not correctly added to the map sm.states[\"mock state\"]: %v; want: %v", sm.states["mock state"], state)
	}

	// test adding a new state with the same new overwrites the old state
	state2 := &MockDefaultState{}
	sm.AddState("mock state", state2)
	if sm.states["mock state"] != state2 {
		t.Errorf("AddState() state was not correctly added to the map sm.states[\"mock state\"]: %v; want %v", sm.states["mock state"], state2)
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 50)
}

func TestStateManager_DeleteState(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())
	state := &MockState{}
	sm.AddState("mock state", state)

	// Test no errors when deleting the state
	err := sm.DeleteState("mock state")
	if err != nil {
		t.Errorf("DeleteState() error: %v; want nil", err)
	}

	// Check that the state was removed
	if _, ok := sm.states["mock state"]; ok {
		t.Errorf("DeleteState() did not remove state from the map")
	}

	// Try to delete a state that doesn't exist
	err = sm.DeleteState("nonexistent state")
	if err == nil {
		t.Errorf("DeleteState() error: nil; want: non-nil")
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 50)
}

func TestStateManager_GetState(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())
	state := &MockState{}
	sm.AddState("mock state", state)

	// Test GetState returns the correct state
	gotState, err := sm.GetState("mock state")
	if err != nil {
		t.Errorf("GetState() error: %v; want: nil", err)
	}
	if gotState != state {
		t.Errorf("GetState() returned incorrect state gotState: %v; want: %v", gotState, state)
	}
	if gotState == nil {
		t.Error("GetState() returned nil state; want: non-nil")
	}

	// Test error thrown when non-existent state passed
	gotState, err = sm.GetState("non-existent state")
	if err == nil {
		t.Error("GetState() error: nil; want: non-nil")
	}
	if gotState != nil {
		t.Errorf("GetState() gotState: %v; want: nil", gotState)
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 50)
}

func TestStateManager_CurrentState(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())

	// Check current state type is InitialState before setting state
	currentState := sm.CurrentState()
	switch ty := currentState.(type) {
	case *InitialState:
	default:
		t.Errorf("CurrentState() type before setting state: %v; want: *InitialState", ty)
	}

	// check current state is correct state after setting state once
	state := &MockState{}
	sm.AddState("mock state", state)
	sm.SetState(state)
	if currentState := sm.CurrentState(); currentState != state {
		t.Errorf("CurrentState() after setting state: %v; want: %v", currentState, state)
	}

	state2 := &MockState{}
	sm.AddState("mock state2", state2)
	sm.SetState(state2)

	// Check current state is correct state after changing state
	if currentState := sm.CurrentState(); currentState != state2 {
		t.Errorf("CurrentState() after changing state: %v; want: %v", currentState, state2)
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 50)
}

func TestStateManager_PreviousState(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())

	// Check previous state type is InitialState before setting state
	prevState := sm.PreviousState()
	switch ty := prevState.(type) {
	case *InitialState:
	default:
		t.Errorf("PreviousState() type before setting state: %v; want: *InitialState", ty)
	}

	// Check previous state type is InitialState after setting state once
	state := &MockState{}
	sm.AddState("mock state", state)
	sm.SetState(state)
	prevState = sm.PreviousState()
	switch ty := prevState.(type) {
	case *InitialState:
	default:
		t.Errorf("PreviousState() type after setting state once: %v; want: *InitialState", ty)
	}

	// Check previous state is correct after setting state twice
	state2 := &MockState{}
	sm.AddState("mock state2", state2)
	sm.SetState(state2)
	if prevState := sm.PreviousState(); prevState != state {
		t.Errorf("PreviousState() after setting state: %v; want: %v", prevState, state)
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 50)
}

type MockState_SetState struct {
	MockDefaultState
	entered bool
	exited  bool
}

func (s *MockState_SetState) Enter(prevState State) {
	s.entered = true
}

func (s *MockState_SetState) Exit(nextState State) {
	s.exited = true
}

func TestStateManager_SetState(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())
	state1 := &MockState_SetState{
		entered: false,
		exited:  false,
	}
	sm.AddState("mock state", state1)
	state2 := &MockState_SetState{
		entered: false,
		exited:  false,
	}
	sm.AddState("mock state2", state2)

	t.Run("SetState before Run", func(t *testing.T) {
		// Check state1 Enter was called and Exit was not called
		sm.SetState(state1)
		if state1.entered {
			t.Errorf("SetState() called Enter on setting the new state when state not running")
		}
		if state1.exited {
			t.Errorf("SetState() called Exit on setting the new state when state not running")
		}

		// Check state1 Exit was called and state2 Enter was called
		sm.SetState(state2)
		if state1.exited {
			t.Errorf("SetState() called Exit on the previous state when state not running")
		}
		if state2.entered {
			t.Errorf("SetState() called Enter on the new state when state not running")
		}

		// Check that currentState and prevState were updated correctly
		switch ty := sm.CurrentState().(type) {
		case *InitialState:
			t.Errorf("SetState() updated current state when not running got: %v; want: *InitialState", ty)
		default:
		}
		switch ty := sm.PreviousState().(type) {
		case *InitialState:
			t.Errorf("SetState() updated previous state when not running got: %v; want: *InitialState", ty)
		default:
		}
	})

	t.Run("SetState after Run", func(t *testing.T) {
		sm.Run()
		time.Sleep(time.Millisecond * 50)
		// Check state1 Enter was called and Exit was not called
		sm.SetState(state1)
		if !state1.entered {
			t.Errorf("SetState() did not call Enter on setting the new state")
		}
		if state1.exited {
			t.Errorf("SetState() called Exit on setting the new state")
		}

		// Check state1 Exit was called and state2 Enter was called
		sm.SetState(state2)
		if !state1.exited {
			t.Errorf("SetState() did not call Exit on the previous state")
		}
		if !state2.entered {
			t.Errorf("SetState() did not call Enter on the new state")
		}

		// Check that currentState and prevState were updated correctly
		if sm.CurrentState() != state2 {
			t.Errorf("SetState() did not update current state correctly CurrentState: %v; want: %v", sm.CurrentState(), state2)
		}
		if sm.PreviousState() != state1 {
			t.Errorf("SetState() did not update previous state correctly PreviousState: %v; want: %v", sm.PreviousState(), state1)
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})
}

type MockState_ForceState struct {
	MockDefaultState
	entered bool
	exited  bool
}

func (s *MockState_ForceState) Enter(prevState State) {
	s.entered = true
}

func (s *MockState_ForceState) Exit(nextState State) {
	s.exited = true
}

func TestStateManager_ForceState(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1)

	state1 := &MockState_ForceState{}
	state2 := &MockState_ForceState{}
	sm.ForceState(state1)
	sm.ForceState(state2)

	// Check states not entered or exited despite state manager running
	if state1.entered || state1.exited || state2.entered {
		t.Errorf("ForceState() entered or exited state when state manager was running")
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 50)
}

type MockState_Run struct {
	MockDefaultState
	updateCalled bool
}

func (s *MockState_Run) Update(ctx context.Context) {
	s.updateCalled = true
}

type MockEvent_Run struct {
	processCalled bool
}

func (e *MockEvent_Run) Process(ctx context.Context) error {
	e.processCalled = true
	return nil
}

func TestStateManager_Run(t *testing.T) {
	var got interface{}
	var want interface{}
	sms := StartStateManagement()

	t.Run("normal usage", func(t *testing.T) {
		// Setup
		sm := sms.NewStateManager(1, WithRunTypeNoRun())
		state := &MockState_Run{}
		sm.AddState("mock state", state)

		// Check run type was set to no run
		got = sm.runType
		want = withoutRun
		if got != want {
			t.Errorf("WithRunTypeNoRun() didn't set run type correctly | got: %v, want: %v", got, want)
		}

		// Send event before setting state and run is called
		event1 := &MockEvent_Run{}
		sm.SendEvent(event1)
		time.Sleep(time.Millisecond * 30) // let event be processed

		// Check that Process wasn't called
		if event1.processCalled {
			t.Errorf("Run() called Process on the event when state was nil")
		}

		sm.SetState(state)
		err := sm.Run()
		if err != nil {
			t.Errorf("Run() returned err when statemanager was not previously running | got: %v, want: nil", err)
		}
		time.Sleep(200 * time.Millisecond) // let state manager start running

		// Check run type was set to default run
		got = sm.runType
		want = defaultUpdateInterval
		if got != want {
			t.Errorf("Run() didn't set run type correctly | got: %v, want: %v", got, want)
		}

		// Check that Update was called
		if !state.updateCalled {
			t.Errorf("Run() did not call Update on the current state")
		}

		// Send event1 again
		sm.SendEvent(event1)
		time.Sleep(time.Millisecond * 30) // let event be processed

		// Check that Process was called
		if !event1.processCalled {
			t.Errorf("Run() did not call Process on the event")
		}

		// Check that Update is not called again after cancellation
		sm.Pause()
		time.Sleep(time.Millisecond * 30) // let go routine shut down
		state.updateCalled = false
		time.Sleep(time.Millisecond * 30) // let update be called if still running
		if state.updateCalled {
			t.Errorf("Run() called Update after cancellation")
		}

		// Check that event is not Processed again after cancellation
		event2 := &MockEvent_Run{}
		sm.SendEvent(event2)
		time.Sleep(time.Millisecond * 30) // let event be processed if still running
		if event2.processCalled {
			t.Errorf("Run() called HandleEvent after cancellation")
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})

	t.Run("call Run twice", func(t *testing.T) {
		// Setup
		sm := sms.NewStateManager(1, WithRunTypeNoRun())

		err := sm.Run()
		if err != nil {
			t.Errorf("Run() returned error when statemanager was not previously running | got: %v, want: nil", err)
		}
		err = sm.Run()
		if err == nil {
			t.Errorf("Run() didn't return error when statemanager was already running | got: nil, want: non-nil")
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})

	t.Run("call Run with custom interval", func(t *testing.T) {
		// Setup
		state := &MockState_Run{}
		sm := sms.NewStateManager(1, WithRunTypeNoRun(), WithInitialState(state), WithAddState("test state", state))

		// Check runType was set to default run before running
		got = sm.runType
		want = withoutRun
		if got != want {
			t.Errorf("WithRunTypeNoRun() didn't set run type correctly | got: %v, want: %v", got, want)
		}

		// Check updateInterval was set to default before running
		got = sm.updateInterval
		want = time.Nanosecond
		if got != want {
			t.Errorf("WithRunTypeNoRun() didn't set default updateInterval correctly | got: %v, want: %v", got, want)
		}

		// Ensure Update not called before running
		if state.updateCalled {
			t.Errorf("WithRunTypeNoRun() called Update before statemanager running")
		}

		testCustomInterval := time.Millisecond * 300
		err := sm.Run(testCustomInterval)
		if err != nil {
			t.Errorf("Run(up) returned error when statemanager was not previously running | got: %v, want: nil", err)
		}

		// Ensure update not called "immediately"
		time.Sleep(time.Nanosecond)
		if state.updateCalled {
			t.Errorf("Run(updateInterval) called Update immediately (not after interval)")
		}

		// Ensure update not called 20 milliseconds before interval
		time.Sleep(testCustomInterval - (time.Millisecond * 30))
		if state.updateCalled {
			t.Errorf("Run(updateInterval) called Update before interval")
		}
		// Ensure update called after interval
		time.Sleep(time.Millisecond * 30)
		if !state.updateCalled {
			t.Errorf("Run(updateInterval) did not call Update after interval passed")
		}

		// Check runType was set to custom type run after running
		got = sm.runType
		want = customUpdateInterval
		if got != want {
			t.Errorf("Run(updateInterval) didn't set run type correctly | got: %v, want: %v", got, want)
		}

		// Check updateInterval was set to default before running
		got = sm.updateInterval
		want = testCustomInterval
		if got != want {
			t.Errorf("Run(updateInterval) didn't set custom updateInterval correctly | got: %v, want: %v", got, want)
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})

	t.Run("too many args passed to updateInterval", func(t *testing.T) {
		sm := sms.NewStateManager(1, WithRunTypeNoRun())
		err := sm.Run(time.Millisecond, time.Nanosecond)
		if err == nil {
			t.Errorf("Run(updateInterval) did not return error when too many args passed to 'updateInterval' | got: nil, want: non-nil")
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 50)
	})
}

func TestStateManager_RunBeforeSetState(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1)
	buf := new(bytes.Buffer)
	sms.SetErrorLogger(buf)

	// Send event before setting state and run is called
	event1 := &MockEvent_Run{}
	sm.SendEvent(event1)
	time.Sleep(time.Millisecond * 30) // let event be processed
	if event1.processCalled {
		t.Errorf("Run() called Process before state was set")
	}

	// Try setting state and sending event again
	testState := &MockState_Run{}
	sm.SetState(testState)
	time.Sleep(50 * time.Millisecond) // let setstate be processed
	sm.SendEvent(event1)
	time.Sleep(time.Millisecond * 30) // let event be processed
	if !event1.processCalled {
		t.Errorf("Run() didn't call Process after state was set")
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}

func TestStateManager_RunContinuousUpdate(t *testing.T) {
	var err error
	sms := StartStateManagement()

	t.Run("normal usage", func(t *testing.T) {
		state := &MockState_Run{}
		sm := sms.NewStateManager(1, WithRunTypeNoRun(), WithInitialState(state), WithAddState("test state", state))
		time.Sleep(time.Millisecond * 100)
		if state.updateCalled {
			t.Errorf("WithRunTypeNoRun() update called when not running")
		}
		err = sm.RunContinuousUpdate()
		if err != nil {
			t.Errorf("RunContinuousUpdate() error on normal use | got: %v, want: nil", err)
		}
		// should be continuous but needs sleep time to start goroutine
		time.Sleep(time.Nanosecond)
		if !state.updateCalled {
			t.Errorf("RunContinuousUpdate() didn't call Update when running")
		}
		got := sm.runType
		want := continuousUpdate
		if got != want {
			t.Errorf("RunWithContinuousUpdate() didn't set runType correctly | got: %v, want: %v", got, want)
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})

	t.Run("error from calling twice", func(t *testing.T) {
		state := &MockState_Run{}
		sm := sms.NewStateManager(1, WithRunTypeNoRun(), WithInitialState(state), WithAddState("test state", state))
		time.Sleep(time.Millisecond * 100)
		if state.updateCalled {
			t.Errorf("WithRunTypeNoRun() update called when not running")
		}
		err = sm.RunContinuousUpdate()
		if err != nil {
			t.Errorf("RunContinuousUpdate() error on normal use | got: %v, want: nil", err)
		}
		err = sm.RunContinuousUpdate()
		if err == nil {
			t.Errorf("RunContinuousUpdate() didn't return error when already running | got: nil, want: non-nil")
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})

	t.Run("error from calling after different Run method", func(t *testing.T) {
		state := &MockState_Run{}
		sm := sms.NewStateManager(1, WithRunTypeNoRun(), WithInitialState(state), WithAddState("test state", state))
		time.Sleep(time.Millisecond * 100)
		if state.updateCalled {
			t.Errorf("WithRunTypeNoRun() update called when not running")
		}
		err = sm.Run()
		if err != nil {
			t.Errorf("Run() error on normal use | got: %v, want: nil", err)
		}
		err = sm.RunContinuousUpdate()
		if err == nil {
			t.Errorf("RunContinuousUpdate() didn't return error when already running | got: nil, want: non-nil")
		}

		got := sm.runType
		want := defaultUpdateInterval
		if got != want {
			t.Errorf("RunWithContinuousUpdate() overwrote runType when it was returned with err | got: %v, want: %v", got, want)
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})
}

func TestStateManager_RunWithoutUpdate(t *testing.T) {
	var err error
	sms := StartStateManagement()

	t.Run("normal usage", func(t *testing.T) {
		state := &MockState_Run{}
		sm := sms.NewStateManager(1, WithRunTypeNoRun(), WithInitialState(state), WithAddState("test state", state))
		time.Sleep(time.Millisecond * 100)
		if state.updateCalled {
			t.Errorf("WithRunTypeNoRun() update called when not running")
		}
		err = sm.RunWithoutUpdate()
		if err != nil {
			t.Errorf("RunWithoutUpdate() error on normal use | got: %v, want: nil", err)
		}
		// should be continuous but needs sleep time to start goroutine
		time.Sleep(time.Millisecond * 50)
		if state.updateCalled {
			t.Errorf("RunWithoutUpdate() called Update when running")
		}
		got := sm.runType
		want := withoutUpdate
		if got != want {
			t.Errorf("RunWithoutUpdate() didn't set runType correctly | got: %v, want: %v", got, want)
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})

	t.Run("error from calling twice", func(t *testing.T) {
		state := &MockState_Run{}
		sm := sms.NewStateManager(1, WithRunTypeNoRun(), WithInitialState(state), WithAddState("test state", state))
		time.Sleep(time.Millisecond * 100)
		if state.updateCalled {
			t.Errorf("WithRunTypeNoRun() update called when not running")
		}
		err = sm.RunWithoutUpdate()
		if err != nil {
			t.Errorf("RunWithoutUpdate() error on normal use | got: %v, want: nil", err)
		}
		err = sm.RunWithoutUpdate()
		if err == nil {
			t.Errorf("RunWithoutUpdate() didn't return error when already running | got: nil, want: non-nil")
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})

	t.Run("error from calling after different Run method", func(t *testing.T) {
		state := &MockState_Run{}
		sm := sms.NewStateManager(1, WithRunTypeNoRun(), WithInitialState(state), WithAddState("test state", state))
		time.Sleep(time.Millisecond * 100)
		if state.updateCalled {
			t.Errorf("WithRunTypeNoRun() update called when not running")
		}
		err = sm.Run()
		if err != nil {
			t.Errorf("Run() error on normal use | got: %v, want: nil", err)
		}
		err = sm.RunWithoutUpdate()
		if err == nil {
			t.Errorf("RunWithoutUpdate() didn't return error when already running | got: nil, want: non-nil")
		}

		got := sm.runType
		want := defaultUpdateInterval
		if got != want {
			t.Errorf("RunWithoutUpdate() overwrote runType when it was returned with err | got: %v, want: %v", got, want)
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})
}

type MockState_Pause struct {
	MockDefaultState
	entered bool
}

func TestStateManager_Pause(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1)
	time.Sleep(time.Millisecond * 50)

	// Cancel the context
	sm.Pause()

	// Check that state does not en
	state := &MockState_Pause{}
	sm.SetState(state)
	time.Sleep(time.Millisecond * 50)
	if state.entered {
		t.Errorf("state entered after Pause")
	}

	// Check that the context was cancelled
	if sm.ctx.Err() == nil {
		t.Errorf("Pause() did not cancel the context")
	}

	// Call Cancel again
	sm.Pause()

	// Check that the context is still cancelled
	if sm.ctx.Err() == nil {
		t.Errorf("Pause() did not maintain the cancellation of the context")
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}

type MockState_Reset struct {
	MockDefaultState
	entered bool
}

func (s *MockState_Reset) Enter(prevState State) {
	s.entered = true
}

func TestStateManager_Reset(t *testing.T) {
	var actual interface{}
	var expected interface{}
	sms := StartStateManagement()
	sm := sms.NewStateManager(1)
	state1 := &MockDefaultState{}
	state2 := &MockDefaultState{}
	state3 := &MockState_Reset{}
	sm.SetState(state1)
	sm.SetState(state2)

	// Check currentState and prevState revert back to InitialState after Reset
	sm.Reset()
	time.Sleep(time.Millisecond * 50)
	switch actual := sm.CurrentState().(type) {
	case *InitialState: // expected
	default:
		t.Errorf("Reset() did not reset currentState correctly | got: %v; want: *InitialState", actual)
	}
	switch actual := sm.PreviousState().(type) {
	case *InitialState: // expected
	default:
		t.Errorf("Reset() did not reset prevState correctly | got: %v; want: *InitialState", actual)
	}

	// Check SetState still changes state after Reset
	sm.SetState(state3)
	actual = sm.CurrentState()
	expected = state3
	if expected != actual {
		t.Errorf("Reset() did not set state properly after Reset | got: %v; want: %v", actual, expected)
	}

	// Check SetState does not call state.Enter after Reset
	actual = state3.entered
	expected = false
	if expected != actual {
		t.Errorf("Reset() called state.Enter on setting state after Reset")
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}

func TestStateManager_Restart(t *testing.T) {
	var got interface{}
	var want interface{}
	sms := StartStateManagement()

	t.Run("normal use", func(t *testing.T) {
		event := &MockEvent_Run{}
		state := &MockState_Run{}
		sm := sms.NewStateManager(1, WithInitialState(state), WithAddState("test state", state))
		sm.Pause()
		time.Sleep(time.Millisecond * 30)
		sm.SendEvent(event)
		time.Sleep(time.Millisecond * 30)
		if event.processCalled {
			t.Errorf("SendEvent() event Process was called after Pause")
		}
		sm.Restart()
		sm.SendEvent(event)
		time.Sleep(time.Millisecond * 30)
		if !event.processCalled {
			t.Errorf("Restart() did not call Process after restarting")
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})

	t.Run("called restart while running", func(t *testing.T) {
		event1 := &MockEvent_Run{}
		event2 := &MockEvent_Run{}
		state := &MockState_Run{}
		buf := new(bytes.Buffer)
		testCustomInterval := time.Millisecond * 20
		sms.SetErrorLogger(buf)
		sm := sms.NewStateManager(1, WithInitialState(state), WithAddState("test state", state), WithRunTypeCustomUpdate(testCustomInterval))

		// Verify buf is empty initially
		got = buf.String()
		want = ""
		if got != want {
			t.Errorf("Restart() buf was not empty before restarting | got %v, want: \"\"", got)
		}

		// Verify events process before restart
		sm.SendEvent(event1)
		time.Sleep(time.Millisecond * 30)
		if !event1.processCalled {
			t.Errorf("Restart() event not called before restart")
		}
		time.Sleep(time.Millisecond * 30)
		if !state.updateCalled {
			t.Errorf("Restart() did not call Update before restart")
		}

		// Check that statemanager stop statement was written to buf on Restart
		sm.Restart()
		time.Sleep(time.Millisecond * 50)
		if buf.String() == "" {
			t.Errorf("Restart() did not write to buf on restart")
		}

		// Check statemanager maintained updateInterval and runType
		got = sm.runType
		want = customUpdateInterval
		if got != want {
			t.Errorf("Restart() did not maintain runType after restart | got: %v, want: %v", got, want)
		}
		got = sm.updateInterval
		want = testCustomInterval
		if got != want {
			t.Errorf("Restart() did not maintain updateInterval after restart | got: %v, want: %v", got, want)
		}

		// Make sure events still process
		sm.SendEvent(event2)
		time.Sleep(time.Millisecond * 30)
		if !event2.processCalled {
			t.Errorf("Restart() did not process event after restart")
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})

	t.Run("do nothing calling Restart before running", func(t *testing.T) {
		state := &MockState_Run{}
		buf := new(bytes.Buffer)
		sms.SetErrorLogger(buf)
		sm := sms.NewStateManager(1, WithInitialState(state), WithAddState("test state", state), WithRunTypeNoRun())
		time.Sleep(time.Millisecond * 30)
		sm.Restart()
		time.Sleep(time.Millisecond * 50)

		// Check to make sure no error messages are written
		got = buf.String()
		want = ""
		if got != want {
			t.Errorf("Restart() wrote error message when called before statemanager run | got: %v, want: \"\"", got)
		}

		// Ensure runType maintained
		got = sm.runType
		want = withoutRun
		if got != want {
			t.Errorf("Restart() didn't maintain runType as no run when called before running")
		}

		// Check if state is updating
		if state.updateCalled {
			t.Errorf("Restart() updated state before running")
		}

		// Tear down
		sm.Pause()
		sms.DeleteStateManager(1)
		time.Sleep(time.Millisecond * 30)
	})
}

// CounterEvent is a mock implementation of the Event interface for testing.
// It increments a shared counter when processed.
type MockEvent_SendEvent struct {
	counter *int64
}

func (e *MockEvent_SendEvent) Process(ctx context.Context) error {
	time.Sleep(time.Millisecond * 100)
	atomic.AddInt64(e.counter, 1)
	return nil
}

func TestStateManager_SendEvent(t *testing.T) {
	sms := StartStateManagement()
	state := &MockDefaultState{}
	sm := sms.NewStateManager(1, WithAddState("mock state", state), WithInitialState(state))

	// Shared counter
	var counter int64 = 0

	// Test sending multiple events concurrently
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := &MockEvent_SendEvent{counter: &counter}
			sm.SendEvent(event)
		}()
	}
	wg.Wait()

	// Allow some time for the buffered events to be processed
	time.Sleep(500 * time.Millisecond)

	// Check that all events were processed
	if counter != 200 {
		t.Errorf("SendEvent() did not process all events, counter = %v, want 200", counter)
	}

	// Tear down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}

type MockState_SetErrorLogger struct {
	MockDefaultState
}

func (s *MockState_SetErrorLogger) HandleEvent(ctx context.Context, event Event, responseChan chan interface{}) error {
	switch e := event.(type) {
	case *MockDefaultEvent:
		return fmt.Errorf("HandleEvent called; test error log")
	case *MockEvent_SetErrorLogger:
		err := e.Process(ctx)
		if err != nil {
			return err
		}
	default:
		return nil
	}
	return nil
}

type MockEvent_SetErrorLogger struct {
	MockDefaultEvent
}

func (e *MockEvent_SetErrorLogger) Process(ctx context.Context) error {
	return fmt.Errorf("Process called; test error log")
}

func TestSMSystem_SetErrorLogger(t *testing.T) {
	// Create a new SMSystem
	sms := StartStateManagement()

	// Add some StateManagers to the SMSystem
	testErrorState := &MockState_SetErrorLogger{}
	sm1 := sms.NewStateManager(1, WithRunTypeNoRun(), WithInitialState(testErrorState))
	sm1.AddState("testErrorState", testErrorState)
	sms.NewStateManager(2, WithRunTypeNoRun())

	// Create a buffer to use as the logger output
	buf := new(bytes.Buffer)

	// Set the error logger
	logger := sms.SetErrorLogger(buf)

	// Check that the logger was correctly set for the SMSystem
	if sms.errorLogger != logger {
		t.Errorf("SetErrorLogger() did not correctly set the logger for the SMSystem")
	}

	// Check that the logger was correctly set for each StateManager
	for _, sm := range sms.stateManagers {
		if sm.errorLogger != logger {
			t.Errorf("SetErrorLogger() did not correctly set the logger for a StateManager")
		}
	}

	// Check that error is written to logger when HandleEvent always returns err
	sm1.Run()
	testErrorEvent1 := &MockDefaultEvent{}
	sm1.SendEvent(testErrorEvent1)
	time.Sleep(time.Millisecond * 30)
	if !strings.Contains(buf.String(), "HandleEvent called; test error log") {
		t.Errorf("SetErrorLogger() did not correctly write HandleEvent error message to errorLogger output")
	}

	// Check that error is written to logger when Process always returns err
	testErrorEvent2 := &MockEvent_SetErrorLogger{}
	sm1.SendEvent(testErrorEvent2)
	time.Sleep(time.Millisecond * 30)
	if !strings.Contains(buf.String(), "Process called; test error log") {
		t.Errorf("SetErrorLogger() did not correctly write Process error message to errorLogger output")
	}

	// Tear down
	sm1.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}

type MockState_ReceiveResponse struct {
	MockDefaultState
}

func (s *MockState_ReceiveResponse) HandleEvent(ctx context.Context, event Event, responseChan chan interface{}) error {
	switch event.(type) {
	case *MockEvent1_ReceiveResponse:
		responseChan <- "event occurred"
	case *MockEvent2_ReceiveResponse:
		responseChan <- "mock error event"
	case *MockEvent3_ReceiveResponse:
		responseChan <- "event shouldn't occur"
	}
	return nil
}

type MockEvent1_ReceiveResponse struct {
	MockDefaultEvent
}

type MockEvent2_ReceiveResponse struct {
	MockDefaultEvent
}

type MockEvent3_ReceiveResponse struct {
	MockDefaultEvent
}

func TestStateManager_ReceiveResponse(t *testing.T) {
	// Setup for test
	sms := StartStateManagement()
	testState := &MockState_ReceiveResponse{}
	sm := sms.NewStateManager(1, WithInitialState(testState))
	sm.AddState("testState", testState)

	testEvent1 := &MockEvent1_ReceiveResponse{}
	testEvent2 := &MockEvent2_ReceiveResponse{}
	testEvent3 := &MockEvent3_ReceiveResponse{}

	// Create a channel to signal when we have received all responses
	done := make(chan bool)

	// Start a goroutine that constantly reads from ReceiveResponse
	// Test messages are received in the correct order
	messageReceivedCounter := 0
	go func() {
		for {
			resp, ok := sm.ReceiveResponse()
			if ok {
				messageReceivedCounter++
				respStr := resp.(string)
				if messageReceivedCounter == 1 && respStr != "event occurred" {
					t.Errorf("ReceiveResponse() did not receive correct message from first event message: %v", resp)
					time.Sleep(time.Millisecond * 100)
				} else if messageReceivedCounter == 2 && respStr != "mock error event" {
					t.Errorf("ReceiveResponse() did not receive correct message from second event message: %v", resp)
					time.Sleep(time.Millisecond * 100)
					log.Println("sleeping before break")
				} else if messageReceivedCounter == 2 {
					break
				}
			} else {
				// no response was available, sleep before checking again
				time.Sleep(time.Millisecond * 100)
			}
		}
		// Signal that we have received all responses
		done <- true
	}()

	// Send test events
	sm.SendEvent(testEvent1)
	sm.SendEvent(testEvent2)

	// Wait for the goroutine to signal that it has received all responses
	<-done

	// Shutdown
	sm.Pause()
	time.Sleep(time.Millisecond * 100)

	// Test receiving response after shutdown
	sm.SendEvent(testEvent3)
	time.Sleep(time.Millisecond * 30)
	resp, ok := sm.ReceiveResponse()
	if ok {
		t.Errorf("ReceiveResponse() received message \"%v\" after shutdown", resp)
	}
	sms.DeleteStateManager(1)
}

func TestStateManager_IsRunning(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())

	// Check IsRunning when WithoutRun passed to NewStateManager
	if sm.IsRunning() {
		t.Errorf("IsRunning() returned true before state manager was running")
	}

	sm.Run()
	time.Sleep(time.Millisecond * 20)

	// Check IsRunning after manually calling Run
	if !sm.IsRunning() {
		t.Errorf("IsRunning() returned false after manually calling Run")
	}

	// Tear Down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}

func TestStateManager_WithValue(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())
	key := "key"
	val := "value"
	sm.WithValue(key, val)

	if sm.ctx.Value(key) != val {
		t.Errorf("WithValue() got: %v, want: %v", sm.ctx.Value(key), val)
	}

	// Tear Down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}

func TestStateManager_WithDeadline(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())
	deadline := time.Now().Add(10 * time.Second)
	sm.WithDeadline(deadline)

	d, ok := sm.ctx.Deadline()
	if !ok || !d.Equal(deadline) {
		t.Errorf("WithDeadline() got: %v, want: %v", d, deadline)
	}

	// Tear Down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}

func TestStateManager_WithTimeout(t *testing.T) {
	sms := StartStateManagement()
	sm := sms.NewStateManager(1, WithRunTypeNoRun())
	timeout := 10 * time.Second
	sm.WithTimeout(timeout)

	deadline, ok := sm.ctx.Deadline()
	if !ok || time.Until(deadline) > timeout {
		t.Errorf("WithTimeout() got: %v, want: %v", time.Until(deadline), timeout)
	}

	// Tear Down
	sm.Pause()
	sms.DeleteStateManager(1)
	time.Sleep(time.Millisecond * 30)
}
