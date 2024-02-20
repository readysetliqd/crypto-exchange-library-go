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

func TestNewStateManager(t *testing.T) {
	sms := StartStateManagement()

	// check state manager was initialized
	sm, err := sms.NewStateManager(1)
	if err != nil {
		t.Fatalf("NewStateManager() error: %v; want: nil", err)
	}
	if sm == nil {
		t.Fatalf("NewStateManager(): %v; want: non-nil", sms)
	}

	// check sm was added to the map at the correct key
	if sms.stateManagers[1] != sm {
		t.Errorf("NewStateManager() did not add state manager to the stateManagers map at the correct key")
	}

	// check fields initialized correctly
	if sm.currentState != nil {
		t.Errorf("NewStateManager() currentState: %v; want: nil", sm.currentState)
	}
	if sm.prevState != nil {
		t.Errorf("NewStateManager() prevState: %v; want: nil", sm.prevState)
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

	// check that NewStateManager returns an error when duplicate ID is used
	_, err = sms.NewStateManager(1)
	if err == nil {
		t.Errorf("NewStateManager() with duplicate id err: %v; want: non-nil", err)
	}
}

func TestDeleteStateManager(t *testing.T) {
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

func TestAddState(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
	state := &MockState{}
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
}

func TestDeleteState(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
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
}

func TestGetState(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
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
}

func TestCurrentState(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)

	// Check current state is nil before setting state
	if currentState := sm.CurrentState(); currentState != nil {
		t.Errorf("CurrentState() before setting state: %v; want: nil", currentState)
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
}

func TestPreviousState(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)

	// Check previous state is nil before setting state
	if prevState := sm.PreviousState(); prevState != nil {
		t.Errorf("PreviousState() before setting state: %v; want: nil", prevState)
	}

	// Check previous state is nil after setting state once
	state := &MockState{}
	sm.AddState("mock state", state)
	sm.SetState(state)
	if prevState := sm.PreviousState(); prevState != nil {
		t.Errorf("PreviousState() after setting state: %v; want: nil", prevState)
	}

	// Check previous state is correct after setting state twice
	state2 := &MockState{}
	sm.AddState("mock state2", state2)
	sm.SetState(state2)
	if prevState := sm.PreviousState(); prevState != state {
		t.Errorf("PreviousState() after setting state: %v; want: %v", prevState, state)
	}
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

func TestSetState(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
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

	// Check state1 Enter was called and Exit was not called
	sm.SetState(state1)
	if !state1.entered {
		t.Errorf("SetState() did not call Enter on setting the new state")
	}
	if state1.exited {
		t.Errorf("SetState() called exit on setting the new state")
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

func TestRun(t *testing.T) {
	// Setup
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
	state := &MockState_Run{}
	sm.AddState("mock state", state)
	sm.SetState(state)
	go sm.Run()
	time.Sleep(200 * time.Millisecond) // let state manager start running

	// Check that Update was called
	if !state.updateCalled {
		t.Errorf("Run() did not call Update on the current state")
	}

	// Send event
	event1 := &MockEvent_Run{}
	sm.SendEvent(event1)
	time.Sleep(200 * time.Millisecond) // let event be processed

	// Check that Process was called
	if !event1.processCalled {
		t.Errorf("Run() did not call Process on the event")
	}

	// Check that Update is not called again after cancellation
	sm.Cancel()
	time.Sleep(200 * time.Millisecond) // let go routine shut down
	state.updateCalled = false
	time.Sleep(200 * time.Millisecond) // let update be called if still running
	if state.updateCalled {
		t.Errorf("Run() called Update after cancellation")
	}

	// Check that event is not Processed again after cancellation
	event2 := &MockEvent_Run{}
	sm.SendEvent(event2)
	time.Sleep(200 * time.Millisecond) // let event be processed if still running
	if event2.processCalled {
		t.Errorf("Run() called HandleEvent after cancellation")
	}
}

func TestCancel(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)

	// Cancel the context
	sm.Cancel()

	// Check that the context was cancelled
	if sm.ctx.Err() == nil {
		t.Errorf("Cancel() did not cancel the context")
	}

	// Call Cancel again
	sm.Cancel()

	// Check that the context is still cancelled
	if sm.ctx.Err() == nil {
		t.Errorf("Cancel() did not maintain the cancellation of the context")
	}
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

func TestSendEvent(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
	state := &MockDefaultState{}
	sm.AddState("mock state", state)
	sm.SetState(state)

	go sm.Run()
	time.Sleep(100 * time.Millisecond) // let state manager start running

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

	sm.Cancel()
	time.Sleep(100 * time.Millisecond) // let go routine shut down
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

func TestSetErrorLogger(t *testing.T) {
	// Create a new SMSystem
	sms := StartStateManagement()

	// Add some StateManagers to the SMSystem
	sm1, _ := sms.NewStateManager(1)
	sms.NewStateManager(2)

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
	testErrorState := &MockState_SetErrorLogger{}
	sm1.AddState("testErrorState", testErrorState)
	sm1.SetState(testErrorState)
	go sm1.Run()
	testErrorEvent1 := &MockDefaultEvent{}
	sm1.SendEvent(testErrorEvent1)
	time.Sleep(time.Millisecond * 100)
	if !strings.Contains(buf.String(), "HandleEvent called; test error log") {
		t.Errorf("SetErrorLogger() did not correctly write HandleEvent error message to errorLogger output")
	}

	testErrorEvent2 := &MockEvent_SetErrorLogger{}
	sm1.SendEvent(testErrorEvent2)
	time.Sleep(time.Millisecond * 100)
	if !strings.Contains(buf.String(), "Process called; test error log") {
		t.Errorf("SetErrorLogger() did not correctly write Process error message to errorLogger output")
	}

	// Check that error is written to logger when Process always returns err

	// Shutdown
	sm1.Cancel()
	time.Sleep(time.Millisecond * 100)
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

func TestReceiveResponse(t *testing.T) {
	// Setup for test
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
	testState := &MockState_ReceiveResponse{}
	sm.AddState("testState", testState)
	sm.SetState(testState)
	go sm.Run()

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
	sm.Cancel()
	time.Sleep(time.Millisecond * 100)

	// Test receiving response after shutdown
	sm.SendEvent(testEvent3)
	time.Sleep(time.Millisecond * 100)
	resp, ok := sm.ReceiveResponse()
	if ok {
		t.Errorf("ReceiveResponse() received message \"%v\" after shutdown", resp)
	}
}

func TestWithValue(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
	key := "key"
	val := "value"
	sm.WithValue(key, val)

	if sm.ctx.Value(key) != val {
		t.Errorf("WithValue() got: %v, want: %v", sm.ctx.Value(key), val)
	}
}

func TestWithDeadline(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
	deadline := time.Now().Add(10 * time.Second)
	sm.WithDeadline(deadline)

	d, ok := sm.ctx.Deadline()
	if !ok || !d.Equal(deadline) {
		t.Errorf("WithDeadline() got: %v, want: %v", d, deadline)
	}
}

func TestWithTimeout(t *testing.T) {
	sms := StartStateManagement()
	sm, _ := sms.NewStateManager(1)
	timeout := 10 * time.Second
	sm.WithTimeout(timeout)

	deadline, ok := sm.ctx.Deadline()
	if !ok || time.Until(deadline) > timeout {
		t.Errorf("WithTimeout() got: %v, want: %v", time.Until(deadline), timeout)
	}
}
