package krakenspot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

// TODO test if callbacks can be nil and end user can access channels with go routines
// TODO instead of wiping all subscriptions on disconnect with ctx.Cancel, keep them active and attempt resubscribe
// TODO log errors when missing sequence for ownTrades
// TODO add time check for trade logger to ignore previous trades when SubscribeOwnTrades is called with snapshot

// #region Exported WebSocket connection methods (Connect, Subscribe<>, and Unsubscribe<>)

// Creates both authenticated and public connections to Kraken WebSocket server.
// Use ConnectPublic() or ConnectPrivate() instead if only channel type is needed.
// Accepts arg 'systemStatusCallback' callback function which an end user can
// implement their own logic on handling incoming system status change messages.
//
// Note: Creates authenticated token with AuthenticateWebSockets() method which
// expires within 15 minutes. Strongly recommended to subscribe to at least one
// private WebSocket channel and leave it open.
//
// CAUTION: Passing nil to arg 'systemStatusCallback' without handling system
// status changes elsewhere in your program may result in program crashes or
// invalid messages being pushed to Kraken's server on the occasions where
// their system's status is changed. Ensure you have implemented handling
// system status changes on your own or reinitialize the client with a valid
// systemStatusCallback function
//
// # Enum (possible incoming status message values):
//
// 'status': "online", "maintenance", "cancel_only", "limit_only", "post_only"
//
// # Example Usage:
//
// Example 1: Using systemStatusCallback for graceful exits
//
// Prints state of book every 15 seconds until systemStatus message other than
// "online" is received, then calls UnsubscribeAll() method and shuts down program.
//
//	// Note: error handling omitted throughout
//	// initialize KrakenClient and variables
//	kc, err := ks.NewKrakenClient(os.Getenv("KRAKEN_API_KEY"), os.Getenv("KRAKEN_API_SECRET"), 2, true)
//	depth := uint16(10)
//	pair := "XBT/USD"
//	// exit program gracefully if system status isnt "online"
//	systemStatusCallback := func(status string) {
//		if status != "online" {
//			kc.UnsubscribeAll()
//			os.Exit(0)
//		}
//	}
//	// connect and subscribe
//	err = kc.Connect(systemStatusCallback)
//	err = kc.SubscribeBook(pair, depth, nil)
//	// print asks and bids every 15 seconds
//	ticker := time.NewTicker(time.Second * 15)
//	for range ticker.C {
//		asks, err := kc.ListAsks(pair, depth)
//		bids, err := kc.ListBids(pair, depth)
//		log.Println(asks)
//		log.Println(bids)
//	}
//
// Example 2: Using systemStatusCallback for graceful startup
//
// Starts hypothetical goroutine function named placeBidAskSpread() when system
// is online and stops it when any other systemStatus is received.
//
//	// Note: error handling omitted throughout
//	// initialize KrakenClient and variables
//	kc, err := ks.NewKrakenClient(os.Getenv("KRAKEN_API_KEY"), os.Getenv("KRAKEN_API_SECRET"), 2, true)
//	depth := uint16(10)
//	pair := "XBT/USD"
//	// define your placeBidAskSpread function places a bid and ask every 15 seconds
//	placeBidAskSpread := func(ctx context.Context) {
//		for {
//			select {
//			case <-ctx.Done():
//				return
//			default:
//				// logic to place an order each on best bid and best ask
//				time.Sleep(time.Second * 15)
//			}
//		}
//	}
//	// create a context with cancel
//	ctx, cancel := context.WithCancel(context.Background())
//	// handle system status changes
//	systemStatusCallback := func(status string) {
//		if status == "online" {
//			// if system status is online, start the goroutine and subscribe to the book
//			go placeBidAskSpread(ctx)
//			err = kc.SubscribeBook(pair, depth, nil)
//		} else {
//			// if system status is not online, cancel the context to stop the goroutine and unsubscribe from the book
//			cancel()
//			err = kc.UnsubscribeBook(pair, depth)
//			// create a new context for the next time the system status is online
//			ctx, cancel = context.WithCancel(context.Background())
//		}
//	}
//	// connect
//	err = kc.Connect(systemStatusCallback)
//	// block program from exiting
//	select{}
func (kc *KrakenClient) Connect(systemStatusCallback func(status string)) error {
	err := kc.connectPublic()
	if err != nil {
		return err
	}
	err = kc.connectPrivate()
	if err != nil {
		return err
	}
	kc.WebSocketManager.Mutex.Lock()
	kc.WebSocketManager.SystemStatusCallback = systemStatusCallback
	kc.WebSocketManager.Mutex.Unlock()
	return nil
}

// Connects to the public WebSocket endpoints of the Kraken API and initializes
// WebSocketClient. Calls the unexported helper method connectPrivate to establish
// the connection. If the connection is successful, it sets the SystemStatusCallback
// function. It returns an error if the connection fails.
//
// CAUTION: Passing nil to arg 'systemStatusCallback' without handling system
// status changes elsewhere in your program may result in program crashes or
// invalid messages being pushed to Kraken's server on the occasions where
// their system's status is changed. Ensure you have implemented handling
// system status changes on your own or reinitialize the client with a valid
// systemStatusCallback function
//
// See docstrings for (kc *KrakenClient) Connect() for example usage
func (kc *KrakenClient) ConnectPublic(systemStatusCallback func(status string)) error {
	err := kc.connectPublic()
	if err != nil {
		return err
	}
	kc.WebSocketManager.Mutex.Lock()
	kc.WebSocketManager.SystemStatusCallback = systemStatusCallback
	kc.WebSocketManager.Mutex.Unlock()
	return nil
}

// Helper method that initializes WebSocketClient for public endpoints by
// dialing Kraken's server and starting its message reader.
func (kc *KrakenClient) connectPublic() error {
	kc.WebSocketManager.WebSocketClient = &WebSocketClient{Router: kc.WebSocketManager, IsReconnecting: atomic.Bool{}, ErrorLogger: kc.ErrorLogger}
	kc.WebSocketManager.WebSocketClient.IsReconnecting.Store(false)
	err := kc.WebSocketManager.WebSocketClient.dialKraken(wsPublicURL)
	if err != nil {
		return fmt.Errorf("error dialing kraken public url | %w", err)
	}
	kc.WebSocketManager.WebSocketClient.startMessageReader(wsPublicURL)
	return nil
}

// Connects to the private WebSocket endpoints of the Kraken API and initializes
// WebSocketClient. Calls the unexported helper method connectPrivate to establish
// the connection. If the connection is successful, it sets the SystemStatusCallback
// function. It returns an error if the connection fails.
//
// Note: Creates authenticated token with AuthenticateWebSockets() method which
// expires within 15 minutes. Strongly recommended to subscribe to at least one
// private WebSocket channel and leave it open.
//
// CAUTION: Passing nil to arg 'systemStatusCallback' without handling system
// status changes elsewhere in your program may result in program crashes or
// invalid messages being pushed to Kraken's server on the occasions where
// their system's status is changed. Ensure you have implemented handling
// system status changes on your own or reinitialize the client with a valid
// systemStatusCallback function
//
// See docstrings for (kc *KrakenClient) Connect() for example usage
func (kc *KrakenClient) ConnectPrivate(systemStatusCallback func(status string)) error {
	err := kc.connectPrivate()
	if err != nil {
		return err
	}
	kc.WebSocketManager.Mutex.Lock()
	kc.WebSocketManager.SystemStatusCallback = systemStatusCallback
	kc.WebSocketManager.Mutex.Unlock()
	return nil
}

// Helper method that initializes WebSocketClient for private endpoints by
// getting an authenticated WebSocket token, dialing Kraken's server and starting
// its message reader. Starts a loop to attempt reauthentication if an error is
// encountered during those operations.
func (kc *KrakenClient) connectPrivate() error {
	err := kc.AuthenticateWebSockets()
	if err != nil {
		if errors.Is(err, errNoInternetConnection) {
			kc.ErrorLogger.Printf("encountered error; attempting reauth | %s\n", err.Error())
			kc.reauthenticate()
		} else if errors.Is(err, err403Forbidden) {
			kc.ErrorLogger.Printf("encountered error; attempting reauth | %s\n", err.Error())
			kc.reauthenticate()
		} else {
			kc.ErrorLogger.Printf("unknown error encountered while authenticating WebSockets | %s\n", err.Error())
		}
	}
	kc.WebSocketManager.AuthWebSocketClient = &WebSocketClient{Router: kc.WebSocketManager, Authenticator: kc, IsReconnecting: atomic.Bool{}, ErrorLogger: kc.ErrorLogger}
	kc.WebSocketManager.AuthWebSocketClient.IsReconnecting.Store(false)
	err = kc.WebSocketManager.AuthWebSocketClient.dialKraken(wsPrivateURL)
	if err != nil {
		return fmt.Errorf("error dialing kraken private url | %w", err)
	}
	kc.WebSocketManager.AuthWebSocketClient.startMessageReader(wsPrivateURL)
	return nil
}

// Iterates through all open public and private subscriptions and sends an
// unsubscribe message to Kraken's WebSocket server for each. Accepts 0 or 1
// optional arg 'reqID' request ID to send with all Unsubscribe<channel> methods
//
// # Example Usage:
//
//	err := kc.UnsubscribeAll()
func (ws *WebSocketManager) UnsubscribeAll(reqID ...string) error {
	var err error
	if len(reqID) > 1 {
		return fmt.Errorf("%w: expected 0 or 1", ErrTooManyArgs)
	}

	ws.SubscriptionMgr.Mutex.Lock()
	defer ws.SubscriptionMgr.Mutex.Unlock()

	// iterate over all active subscriptions stored in SubscriptionMgr and call
	// corresponding Unsubscribe<channel> method
	for channelName := range ws.SubscriptionMgr.PrivateSubscriptions {
		switch channelName {
		case "ownTrades":
			if len(reqID) > 0 {
				err = ws.UnsubscribeOwnTrades(UnsubscribeOwnTradesReqID(reqID[0]))
			} else {
				err = ws.UnsubscribeOwnTrades()
			}
			if err != nil {
				return fmt.Errorf("error unsubscribing from owntrades | %w", err)
			}
		case "openOrders":
			if len(reqID) > 0 {
				err = ws.UnsubscribeOpenOrders(UnsubscribeOpenOrdersReqID(reqID[0]))
			} else {
				err = ws.UnsubscribeOpenOrders()
			}
			if err != nil {
				return fmt.Errorf("error unsubscribing from openorders | %w", err)
			}
		default:
			return fmt.Errorf("unknown channel name %s", channelName)
		}
	}
	for channelName, pairMap := range ws.SubscriptionMgr.PublicSubscriptions {
		switch {
		case channelName == "ticker":
			for pair := range pairMap {
				if len(reqID) > 0 {
					err = ws.UnsubscribeTicker(pair, ReqID(reqID[0]))
				} else {
					err = ws.UnsubscribeTicker(pair)
				}
				if err != nil {
					return fmt.Errorf("error unsubscribing from ticker | %w", err)
				}
			}
		case channelName == "trade":
			for pair := range pairMap {
				if len(reqID) > 0 {
					err = ws.UnsubscribeTrade(pair, ReqID(reqID[0]))
				} else {
					err = ws.UnsubscribeTrade(pair)
				}
				if err != nil {
					return fmt.Errorf("error unsubscribing from trade | %w", err)
				}
			}
		case channelName == "spread":
			for pair := range pairMap {
				if len(reqID) > 0 {
					err = ws.UnsubscribeSpread(pair, ReqID(reqID[0]))
				} else {
					err = ws.UnsubscribeSpread(pair)
				}
				if err != nil {
					return fmt.Errorf("error unsubscribing from spread | %w", err)
				}
			}
		case strings.HasPrefix(channelName, "ohlc"):
			_, intervalStr, _ := strings.Cut(channelName, "-")
			interval, err := strconv.ParseUint(intervalStr, 10, 16)
			if err != nil {
				return fmt.Errorf("error parsing uint from interval | %w", err)
			}
			for pair := range pairMap {
				if len(reqID) > 0 {
					err = ws.UnsubscribeOHLC(pair, uint16(interval), ReqID(reqID[0]))
				} else {
					err = ws.UnsubscribeOHLC(pair, uint16(interval))
				}
				if err != nil {
					return fmt.Errorf("error unsubscribing from ohlc | %w", err)
				}
			}
		case strings.HasPrefix(channelName, "book"):
			_, depthStr, _ := strings.Cut(channelName, "-")
			depth, err := strconv.ParseUint(depthStr, 10, 16)
			if err != nil {
				return fmt.Errorf("error parsing uint from depth | %w", err)
			}
			for pair := range pairMap {
				if len(reqID) > 0 {
					err = ws.UnsubscribeBook(pair, uint16(depth), ReqID(reqID[0]))
				} else {
					err = ws.UnsubscribeBook(pair, uint16(depth))
				}
				if err != nil {
					return fmt.Errorf("error unsubscribing from book | %w", err)
				}
			}
		}
	}
	return nil
}

// Subscribes to "ticker" WebSocket channel for arg 'pair'. May pass a valid
// function to arg 'callback' to dictate what to do with incoming data to the
// channel or pass a nil callback if you choose to handle channels manually.
// Accepts up to one functional options arg 'options' for reqID.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one method or the other.
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	tickerCallback := func(tickerData interface{}) {
//		if msg, ok := tickerData.(ks.WSTickerResp); ok {
//			log.Println(msg.TickerInfo.Bid)
//		}
//	}
//	err := kc.SubscribeTicker("XBT/USD", tickerCallback)
//	if err != nil {
//		log.Println(err)
//	}
func (ws *WebSocketManager) SubscribeTicker(pair string, callback GenericCallback, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 2 or 3", err)
	}
	// Build payload and subscribe
	channelName := "ticker"
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s"}%s}`, pair, channelName, buffer.String())
	err = ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "ticker" WebSocket channel for arg 'pair'. Accepts up to one
// functional options arg 'options' for reqID.
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	err := kc.UnsubscribeTicker("XBT/USD")
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeTicker(pair string, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 1 or 2", err)
	}
	// Build payload and unsubscribe
	channelName := "ticker"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s"}%s}`, pair, channelName, buffer.String())
	ws.WebSocketClient.Mutex.Lock()
	if ws.WebSocketClient.Conn != nil {
		err := ws.WebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			err = fmt.Errorf("error writing message | %w", err)
			return err
		}
	}
	ws.WebSocketClient.Mutex.Unlock()
	return nil
}

// Subscribes to "ohlc" WebSocket channel for arg 'pair' and specified 'interval'
// in minutes. On subscribing, sends last valid closed candle (had at least one
// trade), irrespective of time. May pass a valid function to arg 'callback'
// to dictate what to do with incoming data to the channel or pass a nil callback
// if you choose to handle channels manually. Accepts up to one functional
// options arg 'options' for reqID.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one method or the other.
//
// # Enum:
//
// 'interval' - 1, 5, 15, 30, 60, 240, 1440, 10080, 21600
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	ohlcCallback := func(ohlcData interface{}) {
//		if msg, ok := ohlcData.(ks.WSOHLCResp); ok {
//			log.Println(msg.OHLC)
//		}
//	}
//	err = kc.SubscribeOHLC("XBT/USD", 5, ohlcCallback)
//	if err != nil {
//		log.Println(err)
//	}
func (ws *WebSocketManager) SubscribeOHLC(pair string, interval uint16, callback GenericCallback, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 3 or 4", err)
	}
	// Build payload and subscribe
	name := "ohlc"
	channelName := fmt.Sprintf("%s-%v", name, interval)
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s", "interval": %v}%s}`, pair, name, interval, buffer.String())
	err = ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "ohlc" WebSocket channel for arg 'pair' and specified 'interval'
// in minutes.  Accepts up to one functional options arg 'options' for reqID.
//
// # Enum:
//
// 'interval' - 1, 5, 15, 30, 60, 240, 1440, 10080, 21600
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	err := kc.UnsubscribeOHLC("XBT/USD", 1)
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeOHLC(pair string, interval uint16, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 2 or 3", err)
	}
	// Build payload and unsubscribe
	channelName := "ohlc"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s", "interval": %v}%s}`, pair, channelName, interval, buffer.String())
	ws.WebSocketClient.Mutex.Lock()
	if ws.WebSocketClient.Conn != nil {
		err := ws.WebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			err = fmt.Errorf("error writing message | %w", err)
			return err
		}
	}
	ws.WebSocketClient.Mutex.Unlock()
	return nil
}

// Subscribes to "trade" WebSocket channel for arg 'pair'. May pass a valid
// function to arg 'callback' to dictate what to do with incoming data to the
// channel or pass a nil callback if you choose to handle channels manually.
// Accepts up to one functional options arg 'options' for reqID.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one method or the other.
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	tradeCallback := func(tradeData interface{}) {
//		if msg, ok := tradeData.(ks.WSTradeResp); ok {
//			log.Println(msg.Trades)
//		}
//	}
//	err = kc.SubscribeTrade("XBT/USD", tradeCallback)
//	if err != nil {
//		log.Println(err)
//	}
func (ws *WebSocketManager) SubscribeTrade(pair string, callback GenericCallback, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 2 or 3", err)
	}
	// Build payload and subscribe
	channelName := "trade"
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s"}%s}`, pair, channelName, buffer.String())
	// ctx, cancel := ... here?
	err = ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "trade" WebSocket channel for arg 'pair'  Accepts up to one
// functional options arg 'options' for reqID.
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	err := kc.UnsubscribeTrade("XBT/USD")
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeTrade(pair string, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 1 or 2", err)
	}
	// Build payload and unsubscribe
	channelName := "trade"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s"}%s}`, pair, channelName, buffer.String())
	ws.WebSocketClient.Mutex.Lock()
	if ws.WebSocketClient.Conn != nil {
		err := ws.WebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			err = fmt.Errorf("error writing message | %w", err)
			return err
		}
	}
	ws.WebSocketClient.Mutex.Unlock()
	return nil
}

// Subscribes to "spread" WebSocket channel for arg 'pair'. May pass either a
// valid function to arg 'callback' to dictate what to do with incoming data
// to the channel or pass a nil callback if you choose to read/handle channel
// data manually. Accepts up to one functional options arg 'options' for reqID.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one method or the other.
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	spreadCallback := func(spreadData interface{}) {
//		if msg, ok := spreadData.(ks.WSSpreadResp); ok {
//			log.Println(msg)
//		}
//	}
//	err = kc.SubscribeSpread("XBT/USD", spreadCallback)
//	if err != nil {
//		log.Println(err)
//	}
func (ws *WebSocketManager) SubscribeSpread(pair string, callback GenericCallback, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 2 or 3", err)
	}
	// Build payload and subscribe
	channelName := "spread"
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s"}%s}`, pair, channelName, buffer.String())
	err = ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "spread" WebSocket channel for arg 'pair'. Accepts up to one
// functional options arg 'options' for reqID.
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	err := kc.UnsubscribeSpread("XBT/USD")
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeSpread(pair string, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 1 or 2", err)
	}
	// Build payload and unsubscribe
	channelName := "spread"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s"}%s}`, pair, channelName, buffer.String())
	ws.WebSocketClient.Mutex.Lock()
	if ws.WebSocketClient.Conn != nil {
		err := ws.WebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			err = fmt.Errorf("error writing message | %w", err)
			return err
		}
	}
	ws.WebSocketClient.Mutex.Unlock()
	return nil
}

// Subscribes to "book" WebSocket channel for arg 'pair' and specified 'depth'
// which is number of levels shown for each bids and asks side. On subscribing,
// the first message received will be the full initial state of the order book.
//
// Accepts function arg 'callback' in which the end user can decide what to do
// with incoming data. If nil is passed to 'callback', the WebSocketManager
// will internally manage and maintain the current state of orderbook for the
// subscription within the KrakenClient instance.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one method or the other.
//
// When nil is passed, access state of the orderbook with the following methods:
//
//	func (ws *WebSocketManager) GetBookState(pair string, depth uint16) (BookState, error)
//
//	func (ws *WebSocketManager) ListAsks(pair string, depth uint16) ([]InternalBookEntry, error)
//
//	func (ws *WebSocketManager) ListBids(pair string, depth uint16) ([]InternalBookEntry, error)
//
// Accepts up to one functional options arg 'options' for reqID.
//
// # Enum:
//
// 'depth' - 10, 25, 100, 500, 1000
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
// Method 1: Package will maintain book state internally by passing nil 'callback'
//
//	// subscribe to book
//	depth := uint16(10)
//	pair := "XBT/USD"
//	err = kc.SubscribeBook(pair, depth, nil)
//	if err != nil {
//		log.Println(err)
//	}
//	// Print list of asks to terminal every 15 seconds
//	ticker := time.NewTicker(time.Second * 15)
//	for range ticker.C {
//		asks, err := kc.ListAsks(pair, depth)
//		if err != nil {
//			log.Println(err)
//		} else {
//			log.Println(asks)
//		}
//	}
//
// Method 2: End user builds and maintains current book state with their own
// custom 'callback' function and helper functions
//
//	type Book struct {
//		Asks []Level
//		Bids []Level
//	}
//	var book Book
//	func initialStateMsg(msg krakenspot.WSOrderBookSnapshot) bool {
//		//...your implementation here
//	}
//	func initializeBook(msg krakenspot.WSOrderBookSnapshot, book *Book) {
//		//...your implementation here
//	}
//	func updateBook(msg krakenspot.WSOrderBookUpdate, book *Book) {
//		//...your implementation here
//	}
//	// call functions for building and updating book as messages are received
//	func bookCallback(bookData interface{}) {
//		if resp, ok := bookData.(krakenspot.WSBookResp); ok {
//			msg := resp.OrderBook
//			if initialStateMsg(msg) { // implement
//				initializeBook(msg, &book)
//			} else {
//				updateBook(msg, book)
//			}
//		}
//	}
//	// subscribe to book
//	depth := uint16(10)
//	pair := "XBT/USD"
//	err := kc.SubscribeBook(pair, depth, bookCallback)
//	if err != nil {
//		log.Println(err)
//	}
//	// Prints book to terminal every 15 seconds
//	ticker := time.NewTicker(time.Second * 15)
//	for range ticker.C {
//		log.Println(book)
//	}
func (ws *WebSocketManager) SubscribeBook(pair string, depth uint16, callback GenericCallback, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 3 or 4", err)
	}
	// Build payload and subscribe
	name := "book"
	channelName := fmt.Sprintf("%s-%v", name, depth)
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s", "depth": %v}%s}`, pair, name, depth, buffer.String())
	if callback == nil {
		callback = ws.bookCallback(channelName, pair, depth)
	}
	err = ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "book" WebSocket channel for arg 'pair' and specified 'depth'
// as number of book entries for each bids and asks. Accepts up to one
// functional options arg 'options' for reqID.
//
// # Enum:
//
// 'depth' - 10, 25, 100, 500, 1000
//
// # Functional Options:
//
//	func ReqID(reqID string) ReqIDOption
//
// # Example Usage:
//
//	err := kc.UnsubscribeBook("XBT/USD", 10)
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeBook(pair string, depth uint16, options ...ReqIDOption) error {
	// Build buffer for functional options
	buffer, err := buildReqIdBuffer(options)
	if err != nil {
		return fmt.Errorf("%w: expected 2 or 3", err)
	}
	// Build payload and unsubscribe
	channelName := "book"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s", "depth": %v}%s}`, pair, channelName, depth, buffer.String())
	ws.WebSocketClient.Mutex.Lock()
	if ws.WebSocketClient.Conn != nil {
		err := ws.WebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			err = fmt.Errorf("error writing message | %w", err)
			return err
		}
	}
	ws.WebSocketClient.Mutex.Unlock()
	return nil
}

// Returns a BookState struct which holds pointers to both Bids and Asks slices
// for current state of book for arg 'pair' and specified 'depth'.
//
// Note: This method should only be called when current state of book is being
// managed by the WebSocketManager within the KrakenClient instance (when
// SubscribeBook() method was called with a nil 'callback' function). This method
// will throw an error if the end user is building and maintaining the order
// book with their own custom callback function passed to 'callback'.
//
// CAUTION: As this method returns pointers to the Asks and Bids fields, any
// modification done directly to Asks or Bids will likely result in an error and
// cause the WebSocketManager to unsubscribe from this channel. If modification
// to these slices is desired, either make a copy from reference use ListAsks()
// and ListBids() instead.
func (ws *WebSocketManager) GetBookState(pair string, depth uint16) (BookState, error) {
	if _, ok := ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair]; !ok {
		return BookState{}, fmt.Errorf("error encountered | book for pair %s and depth %v does not exist; check inputs and/or subscriptions and try again", pair, depth)
	}
	ob := BookState{
		Asks: &ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair].Asks,
		Bids: &ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair].Bids,
	}
	return ob, nil
}

// Returns a copy of the Asks slice which holds the current state of the order
// book for arg 'pair' and specified 'depth'. May be less performant than GetBookState()
// for larger lists ('depth').
//
// Note: This method should only be called when current state of book is being
// managed by the WebSocketManager within the KrakenClient instance (when
// SubscribeBook() method was called with a nil 'callback' function). This method
// will throw an error if the end user is building and maintaining the order
// book with their own custom callback function passed to 'callback'.
func (ws *WebSocketManager) ListAsks(pair string, depth uint16) ([]InternalBookEntry, error) {
	if _, ok := ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair]; !ok {
		return []InternalBookEntry{}, fmt.Errorf("error encountered | book for pair %s and depth %v does not exist; check inputs and/or subscriptions and try again", pair, depth)
	}
	return append([]InternalBookEntry(nil), ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair].Asks...), nil
}

// Returns a copy of the Bids slice which holds the current state of the order
// book for arg 'pair' and specified 'depth'. May be less performant than GetBookState()
// for larger lists ('depth').
//
// Note: This method should only be called when current state of book is being
// managed by the WebSocketManager within the KrakenClient instance (when
// SubscribeBook() method was called with a nil 'callback' function). This method
// will throw an error if the end user is building and maintaining the order
// book with their own custom callback function passed to 'callback'.
func (ws *WebSocketManager) ListBids(pair string, depth uint16) ([]InternalBookEntry, error) {
	if _, ok := ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair]; !ok {
		return []InternalBookEntry{}, fmt.Errorf("error encountered | book for pair %s and depth %v does not exist; check inputs and/or subscriptions and try again", pair, depth)
	}
	return append([]InternalBookEntry(nil), ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair].Bids...), nil
}

// Subscribes to "ownTrades" authenticated WebSocket channel. May pass either
// a valid function to arg 'callback' to dictate what to do with incoming data
// to the channel or pass a nil callback if you choose to read/handle channel
// data manually. Accepts none or many functional options args 'options'.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one method or the other.
//
// # Functional Options:
//
//	// Whether to consolidate order fills by root taker trade(s). If false, all order fills will show separately. Defaults to true if not called.
//	func WithoutConsolidatedTaker()
//	// Whether to send historical feed data snapshot upon subscription. Defaults to true if not called.
//	func WithoutSnapshot()
//	// Attach optional request ID 'reqID' to request
//	func SubscribeOwnTradesReqID(reqID string) SubscribeOwnTradesOption
//
// # Example Usage:
//
//	ownTradesCallback := func(ownTradesData interface{}) {
//		if msg, ok := ownTradesData.(ks.WSOwnTradesResp); ok {
//			log.Println(msg)
//		}
//	}
//	err = kc.SubscribeOwnTrades(ownTradesCallback, krakenspot.WithoutSnapshot())
//	if err != nil {
//		log.Println(err)
//	}
func (ws *WebSocketManager) SubscribeOwnTrades(callback GenericCallback, options ...SubscribeOwnTradesOption) error {
	var subscriptionBuffer bytes.Buffer
	var reqIDBuffer bytes.Buffer
	for _, option := range options {
		switch option.Type() {
		case SubscriptionOption:
			option.Apply(&subscriptionBuffer)
		case PrivateReqIDOption:
			option.Apply(&reqIDBuffer)
		}
	}
	channelName := "ownTrades"
	payload := fmt.Sprintf(`{"event": "subscribe", "subscription": {"name": "%s", "token": "%s"%s}%s}`, channelName, ws.WebSocketToken, subscriptionBuffer.String(), reqIDBuffer.String())
	err := ws.subscribePrivate(channelName, payload, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribePrivate() method | %w", err)
	}
	return nil
}

// Unsubscribes from "ownTrades" WebSocket channel. Accepts up to one functional
// options arg 'options' for reqID.
//
// # Functional Options:
//
//	// Attach optional request ID 'reqID' to request
//	func UnsubscribeOwnTradesReqID(reqID string) UnsubscribeOwnTradesOption
//
// # Example Usage:
//
//	err := kc.UnsubscribeOwnTrades()
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeOwnTrades(options ...UnsubscribeOwnTradesOption) error {
	// Build buffer
	var buffer bytes.Buffer
	if len(options) > 0 {
		if len(options) > 1 {
			return fmt.Errorf("%w: expected 0 or 1", ErrTooManyArgs)
		}
		for _, option := range options {
			option(&buffer)
		}
	}
	// Build payload and send unsubscribe
	channelName := "ownTrades"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "subscription": {"name": "%s", "token": "%s"}%s}`, channelName, ws.WebSocketToken, buffer.String())
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err := ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			return fmt.Errorf("error writing message to auth client | %w", err)
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()
	return nil
}

// Subscribes to "openOrders" authenticated WebSocket channel. May pass either
// a valid function to arg 'callback' to dictate what to do with incoming data
// to the channel or pass a nil callback if you choose to read/handle channel
// data manually. Accepts none or many functional options arg passed to 'options'.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one method or the other.
//
// # Functional Options:
//
//	// Whether to send rate-limit counter in updates  Defaults to false if not called.
//	func WithRateCounter() SubscribeOpenOrdersOption
//	// Attach optional request ID 'reqID' to request
//	func SubscribeOpenOrdersReqID(reqID string) SubscribeOpenOrdersOption
//
// # Example Usage:
//
//	openOrdersCallback := func(openOrdersData interface{}) {
//		if msg, ok := openOrdersData.(ks.WSOpenOrdersResp); ok {
//			log.Println(msg)
//		}
//	}
//	err = kc.SubscribeOpenOrders(openOrdersCallback)
//	if err != nil {
//		log.Println(err)
//	}
func (ws *WebSocketManager) SubscribeOpenOrders(callback GenericCallback, options ...SubscribeOpenOrdersOption) error {
	var subscriptionBuffer bytes.Buffer
	var reqIDBuffer bytes.Buffer
	for _, option := range options {
		switch option.Type() {
		case SubscriptionOption:
			option.Apply(&subscriptionBuffer)
		case PrivateReqIDOption:
			option.Apply(&reqIDBuffer)
		}
	}
	channelName := "openOrders"
	payload := fmt.Sprintf(`{"event": "subscribe", "subscription": {"name": "%s", "token": "%s"%s}%s}`, channelName, ws.WebSocketToken, subscriptionBuffer.String(), reqIDBuffer.String())
	err := ws.subscribePrivate(channelName, payload, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribeprivate method | %w", err)
	}
	return nil
}

// Unsubscribes from "openOrders" authenticated WebSocket channel. Accepts up
// to one functional options arg 'options' for reqID.
//
// # Example Usage:
//
//	err := kc.UnsubscribeOpenOrders()
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeOpenOrders(options ...UnsubscribeOpenOrdersOption) error {
	// Build buffer
	var buffer bytes.Buffer
	if len(options) > 0 {
		if len(options) > 1 {
			return fmt.Errorf("%w: expected 0 or 1", ErrTooManyArgs)
		}
		for _, option := range options {
			option(&buffer)
		}
	}
	// Build payload and send unsubscribe
	channelName := "openOrders"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "subscription": {"name": "%s", "token": "%s"}%s}`, channelName, ws.WebSocketToken, buffer.String())
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err := ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			return fmt.Errorf("error writing message to auth client | %w", err)
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()
	return nil
}

// Starts trade logger which opens (or creates if not exists) file with 'filename'.
// Appends all incoming trades from Kraken's "ownTrades" channel. Suggested to call
// this method before SubscribeOwnTrades(). If errors occur, will log the errors
// to ErrorLogger (defaults to stdout if none is set) and structures an error
// message to log to TradeLogger file.
//
// Note: Calling SubscribeOwnTrades(callback, options ...) without passing
// WithoutSnapshot() to 'options' will result in the latest 50 trades in the
// snapshot to be logged to the log file. This can cause duplicate entries if
// channel is unsubscribed/resubscribed, whether intentionally or due to
// connection errors.
//
// # Example Usage:
//
//	err := kc.StartTradeLogger("todays_trades.log")
//	ownTradesCallback := func(ownTradesData interface{}) {
//		// Don't need to do anything extra here for trades to be logged
//		log.Println("an executed trade was logged by the logger!")
//	}
//	kc.SubscribeOwnTrades(ownTradesCallback, krakenspot.WithoutSnapshot())
//	// deferred function to close "todays_trades.log" in event of panic or shutdown
//	defer func() {
//		if r := recover(); r != nil {
//			fmt.Println("Recovered from panic:", r)
//		}
//		err := kc.StopTradeLogger()
//		if err != nil {
//			log.Fatal(err)
//		}
//	}()
func (ws *WebSocketManager) StartTradeLogger(filename string) error {
	if ws.TradeLogger != nil && ws.TradeLogger.isLogging.Load() {
		return errors.New("tradeLogger is already running")
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("error opening file %s | %w", filename, err)
	}
	ws.Mutex.Lock()
	ws.TradeLogger = &TradeLogger{
		file:   file,
		writer: bufio.NewWriter(file),
		ch:     make(chan map[string]WSOwnTrade, 100),
	}
	ws.TradeLogger.isLogging.Store(true)
	ws.Mutex.Unlock()
	ws.TradeLogger.wg.Add(1)

	go func() {
		defer ws.TradeLogger.wg.Done()

		for trade := range ws.TradeLogger.ch {
			tradeJson, err := json.Marshal(trade)
			if err != nil {
				ws.handleTradeLoggerError("error marshalling WSOwnTrade to JSON", err, trade)
				continue
			}
			_, err = ws.TradeLogger.writer.WriteString(string(tradeJson) + "\n")
			if err != nil {
				ws.handleTradeLoggerError("error writing trade JSON to string", err, trade)
				continue
			}
			err = ws.TradeLogger.writer.Flush()
			if err != nil {
				ws.handleTradeLoggerError("error flushing writer", err, trade)
			}
		}
	}()

	return nil
}

// Waits until go routine finishes writing trades to file then closes TradeLogger
// channel and Tradelogger file. Recommended to call this method explicitly and/or
// inside of a defer func to ensure file close is triggered. Logic within this
// method is behind an atomic.bool check, so calling this method multiple times
// should not cause any issues in your program.
//
// # Example Usage:
//
//	// Create the WebSocketManager and start the TradeLogger
//	err := kc.StartTradeLogger("trades.log")
//	// Defer a function to recover from a panic and call StopTradeLogger() for "trades.log" file closure
//	defer func() {
//		if r := recover(); r != nil {
//			fmt.Println("Recovered from panic:", r)
//		}
//		err := ws.StopTradeLogger()
//		if err != nil {
//			log.Fatal(err)
//		}
//	}()
//	//...Your program logic here...//
//	// Listen for shutdown signals
//	shutdown := make(chan os.Signal, 1)
//	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
//	// Wait for a shutdown signal
//	<-shutdown
func (ws *WebSocketManager) StopTradeLogger() error {
	if ws.TradeLogger == nil {
		return fmt.Errorf("trade logger never initialized, call StartTradeLogger() method")
	}
	if ws.TradeLogger.isLogging.CompareAndSwap(true, false) {
		close(ws.TradeLogger.ch)
		ws.TradeLogger.wg.Wait()
		return ws.TradeLogger.file.Close()
	}
	return fmt.Errorf("trade logger is already stopped")
}

// Helper method to structure error messages and log them to both the TradeLogger
// file and to ErrorLogger.
func (ws *WebSocketManager) handleTradeLoggerError(errorMessage string, err error, trade map[string]WSOwnTrade) {
	ws.ErrorLogger.Println(errorMessage + " | " + err.Error())
	errorLog := map[string]string{
		"error":   err.Error(),
		"message": errorMessage,
		"trade":   fmt.Sprintf("%+v", trade),
	}
	errorLogJson, err := json.Marshal(errorLog)
	if err != nil {
		ws.ErrorLogger.Println("error marshalling errorLog to JSON | ", err)
		return
	}
	_, err = ws.TradeLogger.writer.WriteString(string(errorLogJson) + "\n")
	if err != nil {
		ws.ErrorLogger.Println("error writing errorLogJson to string | ", err)
	}
}

// Starts the open orders manager. Call this method before SubscribeOpenOrders().
// It will build initial state of all currently open orders and maintain it in
// memory as new open order update messages come in.
//
// # Example Usage:
//
//	// Print current state of open orders every 15 seconds
//	err := kc.SubscribeOpenOrders()
//	if err != nil...
//	err := kc.StartOpenOrderManager()
//	if err != nil...
//	ticker := time.NewTicker(time.Second * 15)
//	for range ticker.C {
//		orders := kc.MapOpenOrders()
//		log.Println(orders)
//	}
func (ws *WebSocketManager) StartOpenOrderManager() error {
	if ws.OpenOrdersMgr != nil && ws.OpenOrdersMgr.isTracking.Load() {
		return fmt.Errorf("OpenOrderManager is already running")
	} else if ws.OpenOrdersMgr == nil {
		ws.OpenOrdersMgr = &OpenOrderManager{
			OpenOrders: make(map[string]WSOpenOrder),
			ch:         make(chan (WSOpenOrdersResp)),
			seq:        0,
		}
		ws.OpenOrdersMgr.isTracking.Store(true)
		go ws.startOpenOrderManager()
	} else if ws.OpenOrdersMgr.isTracking.CompareAndSwap(false, true) {
		ws.OpenOrdersMgr.ch = make(chan WSOpenOrdersResp)
		ws.OpenOrdersMgr.seq = 0
		go ws.startOpenOrderManager()
	} else {
		return fmt.Errorf("an error occurred starting OpenOrderManager")
	}

	return nil
}

// Helper function meant to be run as a go routine. Starts a loop that reads
// from ws.OpenOrdersMgr.ch and either builds the initial state of the currently
// open orders or it updates the state removing and adding orders as necessary
func (ws *WebSocketManager) startOpenOrderManager() {
	ws.OpenOrdersMgr.wg.Add(1)
	defer ws.OpenOrdersMgr.wg.Done()

	for data := range ws.OpenOrdersMgr.ch {
		if data.Sequence != ws.OpenOrdersMgr.seq+1 {
			ws.ErrorLogger.Println("improper open orders sequence, shutting down go routine")
			return
		} else {
			ws.OpenOrdersMgr.seq = ws.OpenOrdersMgr.seq + 1
			if ws.OpenOrdersMgr.seq == 1 {
				// Build initial state of open orders
				for _, order := range data.OpenOrders {
					for orderID, orderInfo := range order {
						ws.OpenOrdersMgr.OpenOrders[orderID] = orderInfo
					}
				}
			} else {
				// Update state of open orders
				for _, order := range data.OpenOrders {
					for orderID, orderInfo := range order {
						if orderInfo.Status == "pending" {
							ws.OpenOrdersMgr.OpenOrders[orderID] = orderInfo
						} else if orderInfo.Status == "open" {
							order := ws.OpenOrdersMgr.OpenOrders[orderID]
							order.Status = "open"
							ws.OpenOrdersMgr.OpenOrders[orderID] = order
						} else if orderInfo.Status == "closed" {
							delete(ws.OpenOrdersMgr.OpenOrders, orderID)
						} else if orderInfo.Status == "canceled" {
							delete(ws.OpenOrdersMgr.OpenOrders, orderID)
						}
					}
				}
			}
		}
	}
}

// Stops the internal tracking of current open orders. Closes channel and clears
// the internal OpenOrders map.
func (ws *WebSocketManager) StopOpenOrderManager() error {
	if ws.OpenOrdersMgr == nil {
		return fmt.Errorf("OpenOrderManager never initialized, call StartOpenOrderManager() method")
	}
	if ws.OpenOrdersMgr.isTracking.CompareAndSwap(true, false) {
		close(ws.OpenOrdersMgr.ch)
		ws.OpenOrdersMgr.wg.Wait()
		// clear map
		ws.OpenOrdersMgr.Mutex.Lock()
		ws.OpenOrdersMgr.OpenOrders = nil
		ws.OpenOrdersMgr.Mutex.Unlock()
		return nil
	}
	return fmt.Errorf("OpenOrderManager is already stopped")
}

// Returns a map of the current state of open orders managed by StartOpenOrderManager().
func (ws *WebSocketManager) MapOpenOrders() map[string]WSOpenOrder {
	ws.OpenOrdersMgr.Mutex.RLock()
	defer ws.OpenOrdersMgr.Mutex.RUnlock()
	return ws.OpenOrdersMgr.OpenOrders
}

// Returns a slice of all currently open orders for arg 'pair' sorted ascending
// by price.
func (ws *WebSocketManager) ListOpenOrdersForPair(pair string) ([]map[string]WSOpenOrder, error) {
	var openOrders []map[string]WSOpenOrder
	ws.OpenOrdersMgr.Mutex.RLock()
	for id, order := range ws.OpenOrdersMgr.OpenOrders {
		if order.Description.Pair == pair {
			newOrder := make(map[string]WSOpenOrder)
			newOrder[id] = order
			openOrders = append(openOrders, newOrder)
		}
	}
	ws.OpenOrdersMgr.Mutex.RUnlock()
	sort.Slice(openOrders, func(i, j int) bool {
		for id1 := range openOrders[i] {
			for id2 := range openOrders[j] {
				dec1, err := decimal.NewFromString(openOrders[i][id1].Description.Price)
				if err != nil {
					ws.ErrorLogger.Println("error converting string to decimal")
				}
				dec2, err := decimal.NewFromString(openOrders[j][id2].Description.Price)
				if err != nil {
					ws.ErrorLogger.Println("error converting string to decimal")
				}
				return dec1.Cmp(dec2) < 0
			}
		}
		return false
	})
	return openOrders, nil
}

// LogOpenorders creates or opens a file with arg 'filename' and writes the
// current state of the open orders to the file in json lines format. Accepts
// one optional boolean arg 'overwrite'. If false, clears old file if it exists
// and writes new data to file. If true, appends the current data to the old file
// if it already exists. Defaults to false if no 'overwrite' value is passed.
//
// # Example Usage:
//
// Example 1: Defers LogOpenOrders on main() return or panic
//
//	kc.StartOpenOrderManager()
//	kc.SubscribeOpenOrders(openOrdersCallback)
//	defer func() {
//		err := kc.LogOpenOrders("open_orders.jsonl", true)
//		if err != nil {
//			fmt.Println("Error logging orders:", err)
//		}
//	}()
//	defer func() {
//		if r := recover(); r != nil {
//			panic(r) // re-throw panic after Order logging
//		}
//	}()
//
// Example 2: Create channel and call LogOpenOrders() on shutdown signal
//
//	kc.StartOpenOrderManager()
//	kc.SubscribeOpenOrders(openOrdersCallback)
//	sigs := make(chan os.Signal, 1)
//	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
//	// Start a goroutine that will perform cleanup when the program is interrupted
//	go func() {
//		<-sigs
//		err := kc.LogOpenOrders("open_orders.jsonl", true)
//		if err != nil {
//			fmt.Println("Error logging orders:", err)
//		}
//		os.Exit(0)
//	}()
func (ws *WebSocketManager) LogOpenOrders(filename string, overwrite ...bool) error {
	// Check if OpenOrders manager is valid and running
	if ws.OpenOrdersMgr == nil || !ws.OpenOrdersMgr.isTracking.Load() {
		return fmt.Errorf("open orders manager is not currently running, start with StartOpenOrderManager()")
	}

	// Create or open file
	var file *os.File
	var err error
	if len(overwrite) > 0 {
		if len(overwrite) > 1 {
			return fmt.Errorf("too many args passed to ListOpenOrders(). Expected 1 or 2")
		}
		if !overwrite[0] {
			file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
			if err != nil {
				return fmt.Errorf("error opening file | %w", err)
			}
		} else {
			file, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0666)
			if err != nil {
				return fmt.Errorf("error opening file | %w", err)
			}
			err = file.Truncate(0)
			if err != nil {
				return fmt.Errorf("error truncating file | %w", err)
			}
		}
	} else {
		file, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return fmt.Errorf("error opening file | %w", err)
		}
		err = file.Truncate(0)
		if err != nil {
			return fmt.Errorf("error truncating file | %w", err)
		}
	}
	defer file.Close()

	ws.OpenOrdersMgr.Mutex.RLock()
	defer ws.OpenOrdersMgr.Mutex.RUnlock()

	// Build map of open orders by pair and sorted by price ascending
	logOrders := make(map[string][]map[string]WSOpenOrder)
	for orderID, order := range ws.OpenOrdersMgr.OpenOrders {
		if _, ok := logOrders[order.Description.Pair]; !ok {
			logOrders[order.Description.Pair] = make([]map[string]WSOpenOrder, 0, 60)
			newMap := make(map[string]WSOpenOrder)
			newMap[orderID] = order
			logOrders[order.Description.Pair] = append(logOrders[order.Description.Pair], newMap)
		} else { // pair exists in logOrders
			// Insert new entry by price ascending
			i := sort.Search(len(logOrders[order.Description.Pair]), func(i int) bool {
				var p1 decimal.Decimal
				var p2 decimal.Decimal
				for id := range logOrders[order.Description.Pair][i] {
					p1, err = decimal.NewFromString(logOrders[order.Description.Pair][i][id].Description.Price)
					if err != nil {
						ws.ErrorLogger.Println("error converting decimal to string |", err)
					}
				}
				p2, err := decimal.NewFromString(order.Description.Price)
				if err != nil {
					ws.ErrorLogger.Println("error converting decimal to string |", err)
				}
				return p1.Cmp(p2) >= 0
			})
			logOrders[order.Description.Pair] = append(logOrders[order.Description.Pair], map[string]WSOpenOrder{})
			copy(logOrders[order.Description.Pair][i+1:], logOrders[order.Description.Pair][i:])
			newMap := make(map[string]WSOpenOrder)
			newMap[orderID] = order
			logOrders[order.Description.Pair][i] = newMap
		}
	}
	for _, orders := range logOrders {
		for _, order := range orders {
			jsonOrder, err := json.Marshal(order)
			if err != nil {
				return fmt.Errorf("error marshalling order | %w", err)
			}
			_, err = file.WriteString(string(jsonOrder) + "\n")
			if err != nil {
				return fmt.Errorf("error writing to file | %w", err)
			}
		}
	}
	return nil
}

// #endregion

// #region Exported *WebSocketManager Order methods (addOrder, editOrder, cancelOrder(s))

// Sets OrderStatusCallback to the function passed to arg 'orderStatus'. This
// function determines the behavior of the program when orderStatus type
// messages are received. Recommended to use with a switch case for each of
// the order status types.
//
// # Order Status Types:
//
//	WSAddOrderResp
//
//	WSEditOrderResp
//
//	WSCancelOrderResp
//
//	WSCancelAllResp
//
//	WSCancelAllAfterResp
//
// # Example Usage:
//
//	var orderID string
//	orderStatusCallback := func(orderStatus interface{}) {
//		log.Println(orderStatus)
//		switch s := orderStatus.(type) {
//		case ks.WSAddOrderResp:
//			log.Println(s)
//			if s.Status == "ok" {
//				log.Println("order added! updating orderID")
//				orderID = s.TxID
//			}
//		case ks.WSEditOrderResp:
//			log.Println(s)
//			if s.Status == "ok" {
//				log.Println("order edited! updating orderID")
//				orderID = s.TxID
//			}
//		case ks.WSCancelOrderResp:
//			log.Println(s)
//			if s.Status == "ok" {
//				log.Println("order cancelled!")
//			}
//		}
//	}
//	kc.WebSocketManager.SetOrderStatusCallback(orderStatusCallback)
//	for {
//		select {
//			case <-ticker1.C:
//				kc.WSAddOrder(ks.WSLimit("10000"), "buy", "0.1", pair)
//				ticker1.Stop()
//			case <-ticker2.C:
//				kc.WSEditOrder(orderID, pair, ks.WSNewPrice("8000"))
//				ticker2.Stop()
//			case <-ticker3.C:
//				kc.WSCancelOrder(orderID)
//				ticker3.Stop()
//		}
//	}
func (ws *WebSocketManager) SetOrderStatusCallback(orderStatusCallback func(orderStatus interface{})) {
	ws.Mutex.Lock()
	ws.OrderStatusCallback = orderStatusCallback
	ws.Mutex.Unlock()
}

// Sends an 'orderType' order request on the side 'direction' (buy or sell) of
// amount/qty/size 'volume' for the specified 'pair' passed to args to Kraken's
// WebSocket server. See functions passable to 'orderType' below which may have
// required 'price' args included. Accepts none or many functional options passed
// to arg 'options' which can modify order behavior. Functional options listed
// below may conflict with eachother or have certain argument requirements. Only
// brief docstrings are included here, check each function's individual documentation
// for further related notes and nuance.
//
// # WSOrderType functions:
//
//	// Instantly market orders in at best current prices
//	func WSMarket() WSOrderType
//	// Order type of "limit" where arg 'price' is the level at which the limit order will be placed ...
//	func WSLimit(price string) WSOrderType
//	// Order type of "stop-loss" order type where arg 'price' is the stop loss trigger price ...
//	func WSStopLoss(price string) WSOrderType
//	// Order type of "take-profit" where arg 'price' is the take profit trigger price ...
//	func WSTakeProfit(price string) WSOrderType
//	// Order type of "stop-loss-limit" where arg 'price' is the stop loss trigger price and arg 'price2' is the limit order that will be placed ...
//	func WSStopLossLimit(price, price2 string) WSOrderType
//	// Order type of "take-profit-limit" where arg 'price' is the take profit trigger price and arg 'price2' is the limit order that will be placed ...
//	func WSTakeProfitLimit(price, price2 string) WSOrderType
//	// Order type of "trailing-stop" where arg 'price' is the relative stop trigger price ...
//	func WSTrailingStop(price string) WSOrderType
//	// Order type of "trailing-stop-limit" where arg 'price' is the relative stop trigger price and arg 'price2' is the limit order that will be placed ...
//	func WSTrailingStopLimit(price, price2 string) WSOrderType
//	// Order type of "settle-position". Settles any open margin position of same 'direction' and 'pair' by amount 'volume' ...
//	func WSSettlePosition(leverage string) WSOrderType
//
// # Enums:
//
// 'direction': "buy", "sell"
//
// 'volume': ["0"...] Call Kraken API with *KrakenClient.GetTradeablePairsInfo(pair)
// field 'OrderMin' for specific pairs minimum order size
//
// 'pair': Call Kraken API with *KrakenClient.ListWebsocketNames() method for
// available tradeable pairs WebSocket names
//
// # Functional Options:
//
//	// User reference id 'userref' is an optional user-specified integer id that can be associated with any number of orders ...
//	func WSUserRef(userRef string) WSAddOrderOption
//	// Amount of leverage desired. Defaults to no leverage if function is not called. API accepts string of any number; in practice, must be some integer >= 2 ...
//	func WSLeverage(leverage string) WSAddOrderOption
//	// If true, order will only reduce a currently open position, not increase it or open a new position. Defaults to false if not passed ...
//	func WSReduceOnly() WSAddOrderOption
//	// Add all desired order 'flags' as a single comma-delimited list. Use either this function or call (one or many) the individual flag functions below ...
//	func WSOrderFlags(flags string) WSAddOrderOption
//	// Post-only order (available when ordertype = limit)
//	func WSPostOnly() WSAddOrderOption
//	// Prefer fee in base currency (default if selling) ...
//	func WSFCIB() WSAddOrderOption
//	// Prefer fee in quote currency (default if buying) ...
//	func WSFCIQ() WSAddOrderOption
//	// Disables market price protection for market orders
//	func WSNOMPP() WSAddOrderOption
//
// ~// Order volume expressed in quote currency. This is supported only for market orders ...~
// ~func WSVIQC() WSAddOrderOption~
//
//	// Time-in-force of the order to specify how long it should remain in the order book before being cancelled. Overrides default value with "IOC" (Immediate Or Cancel) ...
//	func WSImmediateOrCancel() WSAddOrderOption
//	// Time-in-force of the order to specify how long it should remain in the order book before being cancelled. Overrides default value with "GTD" (Good Til Date) ...
//	func WSGoodTilDate(expireTime string) WSAddOrderOption
//	// Conditional close of "limit" order type where arg 'price' is the level at which the limit order will be placed ...
//	func WSCloseLimit(price string) WSAddOrderOption
//	// Conditional close of "stop-loss" order type where arg 'price' is the stop loss trigger price ...
//	func WSCloseStopLoss(price string) WSAddOrderOption
//	// Conditional close of "take-profit" order type where arg 'price' is the take profit trigger price ...
//	func WSCloseTakeProfit(price string) WSAddOrderOption
//	// Conditional close of "stop-loss-limit" order type where arg 'price' is the stop loss trigger price and arg 'price2' is the limit order that will be placed ...
//	func WSCloseStopLossLimit(price, price2 string) WSAddOrderOption
//	// Conditional close of "take-profit-limit" order type where arg 'price' is the take profit trigger price and arg 'price2' is the limit order that will be placed ...
//	func WSCloseTakeProfitLimit(price, price2 string) WSAddOrderOption
//	// Conditional close of "trailing-stop" order type where arg 'price' is the relative stop trigger price ...
//	func WSCloseTrailingStop(price string) WSAddOrderOption
//	// Conditional close of "trailing-stop-limit" order type where arg 'price' is the relative stop trigger price and arg 'price2' is the limit order that will be placed ...
//	func WSCloseTrailingStopLimit(price, price2 string) WSAddOrderOption
//	// Pass RFC3339 timestamp (e.g. 2021-04-01T00:18:45Z) after which the matching engine should reject the new order request to arg 'deadline' ...
//	func WSAddWithDeadline(deadline string) WSAddOrderOption
//	// Validates inputs only. Does not submit order. Defaults to "false" if not called.
//	func WSValidateAddOrder() WSAddOrderOption
//	// Attach optional request ID 'reqID' to request
//	func WSAddOrderReqID(reqID string) WSAddOrderOption
//
// # Example Usage:
//
//	// Sends a post only buy limit order request at price level 42100.20 on Bitcoin for 1.0 BTC. On filling, will open an opposite side sell order at price 44000
//	err := kc.WSAddOrder(krakenspot.WSLimit("42100.20"), "buy", "1.0", "XBT/USD", krakenspot.WSPostOnly(), krakenspot.WSCloseLimit("44000"))
func (ws *WebSocketManager) WSAddOrder(orderType WSOrderType, direction, volume, pair string, options ...WSAddOrderOption) error {
	var buffer bytes.Buffer
	orderType(&buffer)
	for _, option := range options {
		option(&buffer)
	}
	event := "addOrder"
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "type": "%s", "volume": "%s", "pair": "%s"%s}`, event, ws.WebSocketToken, direction, volume, pair, buffer.String())
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err := ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			return fmt.Errorf("error writing message to auth client | %w", err)
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()
	return nil
}

// Sends an edit order request for the order with same arg 'orderID' and 'pair'.
// Must have at least one of WSNewVolume() WSNewPrice() WSNewPrice2() functional
// options args passed to 'options', but may also have many.
//
// Note: OrderID, Userref, and post-only flag will all be reset with the new
// order. Pass WSNewUserRef(userRef) with the old 'userRef' and WSNewPostOnly()
// to 'options' to retain these values.
//
// # Enum:
//
// 'pair': Call Kraken API with *KrakenClient.ListWebsocketNames() method for
// available tradeable pairs WebSocket names
//
// # Functional Options:
//
//	// Field "userref" is an optional user-specified integer id associated with edit request ...
//	func WSNewUserRef(userRef string) WSEditOrderOption
//	// Updates order quantity in terms of the base asset.
//	func WSNewVolume(volume string) WSEditOrderOption
//	// Updates limit price for "limit" orders. Updates trigger price for "stop-loss", "stop-loss-limit", "take-profit", "take-profit-limit", "trailing-stop" and "trailing-stop-limit" orders ...
//	func WSNewPrice(price string) WSEditOrderOption
//	// Updates limit price for "stop-loss-limit", "take-profit-limit" and "trailing-stop-limit" orders ...
//	func WSNewPrice2(price2 string) WSEditOrderOption
//	// Post-only order (available when ordertype = limit). All the flags from the parent order are retained except post-only. Post-only needs to be explicitly mentioned on every edit request.
//	func WSNewPostOnly() WSEditOrderOption
//	// Validate inputs only. Do not submit order. Defaults to false if not called.
//	func WSValidateEditOrder() WSEditOrderOption
//	// Attach optional request ID 'reqID' to request
//	func WSEditOrderReqID(reqID string) WSEditOrderOption
//
// # Example Usage:
//
//	kc.WSEditOrder("O26VH7-COEPR-YFYXLK", "XBT/USD", ks.WSNewPrice("21000"), krakenspot.WSNewPostOnly(), krakenspot.WSValidateEditOrder())
func (ws *WebSocketManager) WSEditOrder(orderID, pair string, options ...WSEditOrderOption) error {
	var buffer bytes.Buffer
	for _, option := range options {
		option(&buffer)
	}
	event := "editOrder"
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "orderid": "%s", "pair": "%s"%s}`, event, ws.WebSocketToken, orderID, pair, buffer.String())
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err := ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			return fmt.Errorf("error writing message to auth client | %w", err)
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()
	return nil
}

// Sends a cancel order request to Kraken's authenticated WebSocket server to
// cancel order with specified arg 'orderID'.
//
// # Example Usage:
//
//	err := kc.WSCancelOrder("O26VH7-COEPR-YFYXLK")
func (ws *WebSocketManager) WSCancelOrder(orderID string) error {
	event := "cancelOrder"
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "txid": ["%s"]}`, event, ws.WebSocketToken, orderID)
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err := ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			return fmt.Errorf("error writing message to auth client | %w", err)
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()
	return nil
}

// Sends a cancel order request to Kraken's authenticated WebSocket server to
// cancel multiple orders with specified orderIDs passed to slice 'orderIDs'.
//
// # Example Usage:
//
//	err := kc.WSCancelOrder([]string{"O26VH7-COEPR-YFYXLK", "OGTT3Y-C6I3P-X2I6HX"})
func (ws *WebSocketManager) WSCancelOrders(orderIDs []string) error {
	event := "cancelOrder"
	ordersJSON, err := json.Marshal(orderIDs)
	if err != nil {
		return fmt.Errorf("error marshalling order ids to json | %w", err)
	}
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "txid": %s}`, event, ws.WebSocketToken, string(ordersJSON))
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err = ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			return fmt.Errorf("error writing message to auth client | %w", err)
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()
	return nil
}

// Sends a request to Kraken's authenticated WebSocket server to cancel all open
// orders including partially filled orders.
//
// # Example Usage:
//
//	err := kc.WSCancelAllOrders()
func (ws *WebSocketManager) WSCancelAllOrders() error {
	event := "cancelAll"
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s"}`, event, ws.WebSocketToken)
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err := ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			return fmt.Errorf("error writing message to auth client | %w", err)
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()
	return nil
}

// Sends a cancelAllOrdersAfter request to Kraken's authenticated WebSocket
// server that activates a countdown timer of 'timeout' number of seconds/
//
// Note: From the Kraken Docs  cancelAllOrdersAfter provides a "Dead Man's
// Switch" mechanism to protect the client from network malfunction, extreme
// latency or unexpected matching engine downtime. The client can send a request
// with a timeout (in seconds), that will start a countdown timer which will
// cancel *all* client orders when the timer expires. The client has to keep
// sending new requests to push back the trigger time, or deactivate the mechanism
// by specifying a timeout of 0. If the timer expires, all orders are cancelled
// and then the timer remains disabled until the client provides a new (non-zero)
// timeout.
//
// The recommended use is to make a call every 15 to 30 seconds, providing a
// timeout of 60 seconds. This allows the client to keep the orders in place in
// case of a brief disconnection or transient delay, while keeping them safe in
// case of a network breakdown. It is also recommended to disable the timer ahead
// of regularly scheduled trading engine maintenance (if the timer is enabled,
// all orders will be cancelled when the trading engine comes back from downtime
// - planned or otherwise).
//
// # Example Usage:
//
// kc.WSCancelAllOrdersAfter("60")
func (ws *WebSocketManager) WSCancelAllOrdersAfter(timeout string) error {
	event := "cancelAllOrdersAfter"
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "timeout": %s}`, event, ws.WebSocketToken, timeout)
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err := ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			return fmt.Errorf("error writing message to auth client | %w", err)
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()
	return nil
}

// #endregion

// #region *WebSocketManager helper methods (subscribe, readers, routers, and connections)

// Helper method for public data subscribe methods to handle initializing
// Subscription, sending payload to server, and starting go routine with
// channels to listen for incoming messages.
func (ws *WebSocketManager) subscribePublic(channelName, payload, pair string, callback GenericCallback) error {
	if !publicChannelNames[channelName] {
		return fmt.Errorf("unknown channel name; check valid depth or interval against enum | %s", channelName)
	}
	if ws.WebSocketClient == nil {
		return fmt.Errorf("websocket client not connected, try Connect() or ConnectPublic()")
	}

	sub := newSub(channelName, pair, callback)

	// check if map is nil and assign Subscription to map
	ws.SubscriptionMgr.Mutex.Lock()
	if ws.SubscriptionMgr.PublicSubscriptions[channelName] == nil {
		ws.SubscriptionMgr.PublicSubscriptions[channelName] = make(map[string]*Subscription)
	}
	ws.SubscriptionMgr.PublicSubscriptions[channelName][pair] = sub
	ws.SubscriptionMgr.Mutex.Unlock()

	// Build payload and send subscription message
	ws.WebSocketClient.Mutex.Lock()
	if ws.WebSocketClient.Conn != nil {
		err := ws.WebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			err = fmt.Errorf("error writing subscription message | %w", err)
			return err
		}
	}
	ws.WebSocketClient.Mutex.Unlock()

	// Start go routine listen for incoming data and call callback functions
	go func() {
		<-sub.ConfirmedChan // wait for subscription confirmed
		for {
			select {
			case data := <-sub.DataChan:
				if sub.DataChanClosed == 0 { // channel is open
					sub.Callback(data)
				}
			case <-sub.DoneChan:
				if sub.DoneChanClosed == 0 { // channel is open
					sub.closeChannels()
					// Delete subscription from map
					ws.SubscriptionMgr.Mutex.Lock()
					delete(ws.SubscriptionMgr.PublicSubscriptions[channelName], pair)
					ws.SubscriptionMgr.Mutex.Unlock()
					return
				}
			case <-ws.WebSocketClient.Ctx.Done():
				if sub.DoneChanClosed == 0 { // channel is open
					sub.closeChannels()
					// Delete subscription from map
					ws.SubscriptionMgr.Mutex.Lock()
					delete(ws.SubscriptionMgr.PublicSubscriptions[channelName], pair)
					ws.SubscriptionMgr.Mutex.Unlock()
					return
				}
			}
		}
	}()
	return nil
}

// Helper method for private data subscribe methods to handle initializing
// Subscription, sending payload to server, and starting go routine with
// channels to listen for incoming messages. Payload must include unexpired
// WebSocket token.
func (ws *WebSocketManager) subscribePrivate(channelName, payload string, callback GenericCallback) error {
	if !privateChannelNames[channelName] {
		return fmt.Errorf("unknown channel name; check valid depth or interval against enum | %s", channelName)
	}
	if ws.AuthWebSocketClient == nil {
		return fmt.Errorf("authenticated client not connected, try Connect() or ConnectPrivate()")
	}

	sub := newSub(channelName, "", callback)

	// Assign Subscription to map
	ws.SubscriptionMgr.Mutex.Lock()
	ws.SubscriptionMgr.PrivateSubscriptions[channelName] = sub
	ws.SubscriptionMgr.Mutex.Unlock()

	// Build payload and send subscription message
	ws.AuthWebSocketClient.Mutex.Lock()
	if ws.AuthWebSocketClient.Conn != nil {
		err := ws.AuthWebSocketClient.Conn.WriteMessage(websocket.TextMessage, []byte(payload))
		if err != nil {
			err = fmt.Errorf("error writing subscription message | %w", err)
			return err
		}
	}
	ws.AuthWebSocketClient.Mutex.Unlock()

	// Start go routine listen for incoming data and call callback functions
	switch channelName {
	case "ownTrades":
		go func() {
			<-sub.ConfirmedChan // wait for subscription confirmed
			for {
				select {
				case data := <-sub.DataChan:
					if sub.DataChanClosed == 0 { // channel is open
						sub.Callback(data)
						if ws.TradeLogger != nil && ws.TradeLogger.isLogging.Load() {
							ownTrades, ok := data.(WSOwnTradesResp)
							if !ok {
								ws.ErrorLogger.Println("error asserting data to WSOwnTradesResp")
							} else {
								for _, trade := range ownTrades.OwnTrades {
									ws.TradeLogger.ch <- trade
								}
							}
						}
					}
				case <-sub.DoneChan:
					if sub.DoneChanClosed == 0 { // channel is open
						sub.closeChannels()
						// Delete subscription from map
						ws.SubscriptionMgr.Mutex.Lock()
						delete(ws.SubscriptionMgr.PrivateSubscriptions, channelName)
						ws.SubscriptionMgr.Mutex.Unlock()
						return
					}
				case <-ws.AuthWebSocketClient.Ctx.Done():
					if sub.DoneChanClosed == 0 { // channel is open
						sub.closeChannels()
						// Delete subscription from map
						ws.SubscriptionMgr.Mutex.Lock()
						delete(ws.SubscriptionMgr.PrivateSubscriptions, channelName)
						ws.SubscriptionMgr.Mutex.Unlock()
						return
					}
				}
			}
		}()
	case "openOrders":
		go func() {
			<-sub.ConfirmedChan // wait for subscription confirmed
			for {
				select {
				case data := <-sub.DataChan:
					if sub.DataChanClosed == 0 { // channel is open
						sub.Callback(data)
						if ws.OpenOrdersMgr != nil && ws.OpenOrdersMgr.isTracking.Load() {
							openOrders, ok := data.(WSOpenOrdersResp)
							if !ok {
								ws.ErrorLogger.Println("error asserting data to WSOpenOrdersResp")
							} else {
								ws.OpenOrdersMgr.ch <- openOrders
							}

						}
					}
				case <-sub.DoneChan:
					if sub.DoneChanClosed == 0 { // channel is open
						sub.closeChannels()
						// Delete subscription from map
						ws.SubscriptionMgr.Mutex.Lock()
						delete(ws.SubscriptionMgr.PrivateSubscriptions, channelName)
						ws.SubscriptionMgr.Mutex.Unlock()
						return
					}
				case <-ws.AuthWebSocketClient.Ctx.Done():
					if sub.DoneChanClosed == 0 { // channel is open
						sub.closeChannels()
						// Delete subscription from map
						ws.SubscriptionMgr.Mutex.Lock()
						delete(ws.SubscriptionMgr.PrivateSubscriptions, channelName)
						ws.SubscriptionMgr.Mutex.Unlock()
						return
					}
				}
			}
		}()
	}
	return nil
}

// Starts a goroutine that continuously reads messages from the WebSocket
// connection. If the message is not a heartbeat message, it routes the message.
func (c *WebSocketClient) startMessageReader(url string) {
	go func() {
		for {
			select {
			case <-c.Ctx.Done():
				return
			default:
				_, msg, err := c.Conn.ReadMessage()
				if err != nil {
					if err != nil {
						if err, ok := err.(net.Error); ok && err.Timeout() {
							c.ErrorLogger.Println("websocket connection timed out, attempting reconnect to url |", url)
							c.Cancel()
							c.Conn.Close()
							if c.IsReconnecting.CompareAndSwap(false, true) {
								c.reconnect(url)
							}
							return
						} else if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseProtocolError, websocket.CloseUnsupportedData, websocket.CloseNoStatusReceived) {
							c.ErrorLogger.Println("unexpected websocket closure, attempting reconnect to url |", url)
							c.Cancel()
							c.Conn.Close()
							if c.IsReconnecting.CompareAndSwap(false, true) {
								c.reconnect(url)
							}
							return
						} else if strings.Contains(err.Error(), "wsarecv") {
							c.ErrorLogger.Println("internet connection lost, attempting reconnect to url |", url)
							c.Cancel()
							c.Conn.Close()
							if c.IsReconnecting.CompareAndSwap(false, true) {
								c.reconnect(url)
							}
							return
						}
						c.ErrorLogger.Println("error reading message |", err)
						continue
					}
				}
				if !bytes.Equal(heartbeat, msg) { // not a heartbeat message
					err := c.Router.routeMessage(msg)
					if err != nil {
						c.ErrorLogger.Println("error routing message |", err)
					}
				} else {
					// reset timeout delay on heartbeat message
					c.Conn.SetReadDeadline(time.Now().Add(time.Second * timeoutDelay))
				}
			}
		}
	}()
}

// Does preliminary unmarshalling incoming message and determines which specific
// route<messageType>Message method to call.
func (ws *WebSocketManager) routeMessage(msg []byte) error {
	var err error
	if msg[0] == '[' { // public or private websocket message
		var dataArray GenericArrayMessage
		err = json.Unmarshal(msg, &dataArray)
		if err != nil {
			err = fmt.Errorf("error unmarshalling message | %w", err)
			return err
		}
		if publicChannelNames[dataArray.ChannelName] {
			if err = ws.routePublicMessage(&dataArray); err != nil {
				return fmt.Errorf("error routing public message | %w", err)
			}
		} else if privateChannelNames[dataArray.ChannelName] {
			if err = ws.routePrivateMessage(&dataArray); err != nil {
				return fmt.Errorf("error routing private message | %w", err)
			}
		} else {
			err = fmt.Errorf("unknown channel name | %s", dataArray.ChannelName)
			return err
		}
	} else if msg[0] == '{' { // general/system messages, subscription status, and order response messages
		var dataObject GenericMessage
		err = json.Unmarshal(msg, &dataObject)
		if err != nil {
			err = fmt.Errorf("error unmarshalling message | %w ", err)
			return err
		}
		if _, ok := orderChannelEvents[dataObject.Event]; ok {
			if err = ws.routeOrderMessage(&dataObject); err != nil {
				return fmt.Errorf("error routing general message | %w", err)
			}
		} else if _, ok := generalMessageEvents[dataObject.Event]; ok {
			if err = ws.routeGeneralMessage(&dataObject); err != nil {
				return fmt.Errorf("error routing general message | %w", err)
			}
		} else {
			return fmt.Errorf("unknown event type")
		}

	} else {
		return fmt.Errorf("unknown message type")
	}
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routePublicMessage(msg *GenericArrayMessage) error {
	switch v := msg.Content.(type) {
	case WSTickerResp:
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChan <- v
		}
	case WSOHLCResp:
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChan <- v
		}
	case WSTradeResp:
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChan <- v
		}
	case WSSpreadResp:
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChan <- v
		}
	case WSBookUpdateResp:
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChan <- v
		}
	case WSBookSnapshotResp:
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].DataChan <- v
		}
	default:
		return fmt.Errorf("cannot route unknown channel name | %s", msg.ChannelName)
	}
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routePrivateMessage(msg *GenericArrayMessage) error {
	switch v := msg.Content.(type) {
	case WSOwnTradesResp:
		if ws.SubscriptionMgr.PrivateSubscriptions[v.ChannelName].DataChanClosed == 0 {
			ws.SubscriptionMgr.PrivateSubscriptions[v.ChannelName].DataChan <- v
		}
	case WSOpenOrdersResp:
		if ws.SubscriptionMgr.PrivateSubscriptions[v.ChannelName].DataChanClosed == 0 {
			ws.SubscriptionMgr.PrivateSubscriptions[v.ChannelName].DataChan <- v
		}
	}
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routeOrderMessage(msg *GenericMessage) error {
	switch v := msg.Content.(type) {
	case WSAddOrderResp:
		if ws.OrderStatusCallback != nil {
			ws.OrderStatusCallback(v)
		}
	case WSEditOrderResp:
		if ws.OrderStatusCallback != nil {
			ws.OrderStatusCallback(v)
		}
	case WSCancelOrderResp:
		if ws.OrderStatusCallback != nil {
			ws.OrderStatusCallback(v)
		}
	case WSCancelAllResp:
		if ws.OrderStatusCallback != nil {
			ws.OrderStatusCallback(v)
		}
	case WSCancelAllAfterResp:
		if ws.OrderStatusCallback != nil {
			ws.OrderStatusCallback(v)
		}
	}
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routeGeneralMessage(msg *GenericMessage) error {
	switch v := msg.Content.(type) {
	case WSSubscriptionStatus:
		switch v.Status {
		case "subscribed":
			//DEBUG testing nil callback
			log.Println(v)
			if publicChannelNames[v.ChannelName] {
				if ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].ConfirmedChanClosed == 0 {
					ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].confirmSubscription()
				}
			} else if privateChannelNames[v.ChannelName] {
				if ws.SubscriptionMgr.PrivateSubscriptions[v.ChannelName].ConfirmedChanClosed == 0 {
					ws.SubscriptionMgr.PrivateSubscriptions[v.ChannelName].confirmSubscription()
				}
			}
		case "unsubscribed":
			if publicChannelNames[v.ChannelName] {
				ws.SubscriptionMgr.PublicSubscriptions[v.ChannelName][v.Pair].unsubscribe()
				if strings.HasPrefix(v.ChannelName, "book") {
					ws.OrderBookMgr.OrderBooks[v.ChannelName][v.Pair].unsubscribe()
				}
			} else if privateChannelNames[v.ChannelName] {
				ws.SubscriptionMgr.PrivateSubscriptions[v.ChannelName].unsubscribe()
			}
		case "error":
			return fmt.Errorf("subscribe/unsubscribe error msg received. operation not completed; check inputs and try again | %s", v.ErrorMessage)
		default:
			return fmt.Errorf("cannot route unknown subscriptionStatus status | %s", v.Status)
		}
	case WSSystemStatus:
		if ws.SystemStatusCallback != nil {
			ws.SystemStatusCallback(v.Status)
		}
	case WSPong:
		ws.ErrorLogger.Println("pong | reqid: ", v.ReqID)
	case WSErrorResp:
		return fmt.Errorf("error message: %s | reqid: %d", v.ErrorMessage, v.ReqID)
	default:
		return fmt.Errorf("cannot route unknown msg type | %s", msg.Event)
	}
	return nil
}

// Attempts reconnect with dialKraken() method. Retries 5 times instantly then
// scales back to attempt reconnect once every ~8 seconds.
func (c *WebSocketClient) reconnect(url string) error {
	go func() {
		t := 1.0
		count := 0
		for {
			err := c.dialKraken(url)
			if err != nil {
				c.ErrorLogger.Printf("encountered error on reconnecting, trying again | %s\n", err)
			} else {
				c.ErrorLogger.Println("reconnect successful")
				c.IsReconnecting.Store(false)
				return
			}
			// attempt reconnect instantly 5 times then backoff to every 8 seconds
			if count < 6 {
				count++
				continue
			}
			if t < 8 {
				t = t * 1.3
			}
			time.Sleep(time.Duration(t) * time.Second)
		}
	}()
	// Reauthenticate WebSocket token if Private
	if url == wsPrivateURL {
		err := c.Authenticator.AuthenticateWebSockets()
		if err != nil {
			if errors.Is(err, errNoInternetConnection) {
				c.ErrorLogger.Printf("encountered error; attempting reauth | %s\n", err.Error())
				c.Authenticator.reauthenticate()
			} else if errors.Is(err, err403Forbidden) {
				c.ErrorLogger.Printf("encountered error; attempting reauth | %s\n", err.Error())
				c.Authenticator.reauthenticate()
			}
			return fmt.Errorf("error authenticating websockets | %w", err)
		}
	}
	c.startMessageReader(url)
	return nil
}

func (kc *KrakenClient) reauthenticate() {
	go func() {
		t := 1.0
		count := 0
		for {
			err := kc.AuthenticateWebSockets()
			if err != nil {
				kc.ErrorLogger.Printf("encountered error on reauthenticating, trying again | %s\n", err)
			} else {
				kc.ErrorLogger.Println("reconnect successful")
				return
			}
			// attempt reconnect instantly 5 times then backoff to every 8 seconds
			if count < 6 {
				continue
			}
			if t < 8 {
				t = t * 1.3
			}
			time.Sleep(time.Duration(t) * time.Second)
		}
	}()
}

func (c *WebSocketClient) dialKraken(url string) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{})
	if err != nil {
		err = fmt.Errorf("error dialing kraken | %w", err)
		return err
	}
	c.Conn = conn
	c.Ctx, c.Cancel = context.WithCancel(context.Background())
	// initialize read deadline to prevent blocking
	c.Conn.SetReadDeadline(time.Now().Add(timeoutDelay * time.Second))
	return nil
}

// #endregion

// #region *Subscription helper methods (confirm, close, unsubscribe, and constructor)

// Completes unsubscribing by sending a message to s.DoneChan.
func (s *Subscription) unsubscribe() {
	s.DoneChan <- struct{}{}
}

// Closes the s.ConfirmedChan to signal that the subscription is confirmed.
func (s *Subscription) confirmSubscription() {
	atomic.StoreInt32(&s.ConfirmedChanClosed, 1)
	close(s.ConfirmedChan)
}

// Safely closes the DataChan and DoneChan with atomic flags set before closing.
func (s *Subscription) closeChannels() {
	atomic.StoreInt32(&s.DataChanClosed, 1)
	close(s.DataChan)
	atomic.StoreInt32(&s.DoneChanClosed, 1)
	close(s.DoneChan)
}

// Helper function to build default new *Subscription data type
func newSub(channelName, pair string, callback GenericCallback) *Subscription {
	return &Subscription{
		ChannelName:         channelName,
		Pair:                pair,
		Callback:            callback,
		DataChan:            make(chan interface{}, 30),
		DoneChan:            make(chan struct{}, 1),
		ConfirmedChan:       make(chan struct{}, 1),
		DataChanClosed:      0,
		DoneChanClosed:      0,
		ConfirmedChanClosed: 0,
	}
}

// #endregion

// #region *InternalOrderBook helper methods (bookCallback, unsubscribe, close, buildInitial, checksum, update)

func (ws *WebSocketManager) bookCallback(channelName, pair string, depth uint16) func(data interface{}) {
	return func(data interface{}) {
		if msg, ok := data.(WSBookUpdateResp); ok { // data is book update message
			ws.OrderBookMgr.OrderBooks[channelName][pair].DataChan <- msg
		} else if msg, ok := data.(WSBookSnapshotResp); ok { // data is book snapshot
			// make book-depth map if not exists
			if _, ok := ws.OrderBookMgr.OrderBooks[channelName]; !ok {
				ws.OrderBookMgr.Mutex.Lock()
				ws.OrderBookMgr.OrderBooks[channelName] = make(map[string]*InternalOrderBook)
				ws.OrderBookMgr.Mutex.Unlock()
			}
			if _, ok := ws.OrderBookMgr.OrderBooks[channelName][pair]; !ok {
				ws.OrderBookMgr.Mutex.Lock()
				ws.OrderBookMgr.OrderBooks[channelName][pair] = &InternalOrderBook{
					DataChan:       make(chan WSBookUpdateResp),
					DoneChan:       make(chan struct{}),
					DataChanClosed: 0,
					DoneChanClosed: 0,
				}
				ws.OrderBookMgr.Mutex.Unlock()
				if err := ws.OrderBookMgr.OrderBooks[channelName][pair].buildInitialBook(&msg.OrderBook); err != nil {
					ws.ErrorLogger.Printf("error building initial state of book; sending unsubscribe msg; try subscribing again | %s\n", err)
					err := ws.UnsubscribeBook(pair, depth)
					if err != nil {
						ws.ErrorLogger.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s\n", err)
					}
					return
				}
				go func() {
					ob := ws.OrderBookMgr.OrderBooks[channelName][pair]
					for {
						select {
						case bookUpdate := <-ob.DataChan:
							if ob.DataChanClosed == 0 {
								ob.Mutex.Lock()
								if len(bookUpdate.Asks) > 0 {
									for _, ask := range bookUpdate.Asks {
										newEntry, err := stringEntryToDecimal(&ask)
										if err != nil {
											ws.ErrorLogger.Printf("error calling stringEntryToDecimal; stopping goroutine and unsubscribing | %s\n", err)
											err := ws.UnsubscribeBook(pair, depth)
											if err != nil {
												ws.ErrorLogger.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s\n", err)
											}
										}
										switch {
										case ask.UpdateType == "r":
											err = ob.replenishEntry(newEntry, &ob.Asks)
										case newEntry.Volume.Equal(decimal.Zero):
											err = ob.deleteAskEntry(newEntry, &ob.Asks)
										default:
											err = ob.updateAskEntry(newEntry, &ob.Asks)
										}
										if err != nil {
											ws.ErrorLogger.Printf("error calling replenish method; stopping goroutine and unsubscribing | %s\n", err)
											err := ws.UnsubscribeBook(pair, depth)
											if err != nil {
												ws.ErrorLogger.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s\n", err)
											}
										}
									}
								}
								if len(bookUpdate.Bids) > 0 {
									for _, bid := range bookUpdate.Bids {
										newEntry, err := stringEntryToDecimal(&bid)
										if err != nil {
											ws.ErrorLogger.Printf("error calling stringEntryToDecimal; stopping goroutine and unsubscribing | %s\n", err)
											err := ws.UnsubscribeBook(pair, depth)
											if err != nil {
												ws.ErrorLogger.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s\n", err)
											}
										}
										switch {
										case bid.UpdateType == "r":
											err = ob.replenishEntry(newEntry, &ob.Bids)
										case newEntry.Volume.Equal(decimal.Zero):
											err = ob.deleteBidEntry(newEntry, &ob.Bids)
										default:
											err = ob.updateBidEntry(newEntry, &ob.Bids)
										}
										if err != nil {
											ws.ErrorLogger.Printf("error calling replenish/update/delete method; stopping goroutine and unsubscribing | %s\n", err)
											err := ws.UnsubscribeBook(pair, depth)
											if err != nil {
												ws.ErrorLogger.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s\n", err)
											}
										}
									}
								}
								ob.Mutex.Unlock()
								// Stop go routine and unsubscribe if checksum does not pass
								checksum, err := strconv.ParseUint(bookUpdate.Checksum, 10, 32)
								if err != nil {
									ws.ErrorLogger.Printf("error parsing checksum uint32; stopping goroutine and unsubscribing | %s\n", err)
									err := ws.UnsubscribeBook(pair, depth)
									if err != nil {
										ws.ErrorLogger.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s\n", err)
									}
									return
								}
								if err := ob.validateChecksum(uint32(checksum)); err != nil {
									ws.ErrorLogger.Printf("error validating checksum; stopping goroutine and unsubscribing | %s\n", err)
									err := ws.UnsubscribeBook(pair, depth)
									if err != nil {
										ws.ErrorLogger.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s\n", err)
									}
									return
								}
							}
						case <-ob.DoneChan:
							ws.closeAndDeleteBook(ob, pair, channelName)
							return
						case <-ws.WebSocketClient.Ctx.Done():
							ws.closeAndDeleteBook(ob, pair, channelName)
							return
						}
					}
				}()
			} else {
				ws.ErrorLogger.Println("unknown data type sent to book callback")
			}
		}
	}
}

// Helper method that closes channels for InternalOrderBook and deletes its
// entries from the internal order book
func (ws *WebSocketManager) closeAndDeleteBook(ob *InternalOrderBook, pair, channelName string) {
	if ob.DoneChanClosed == 0 {
		ob.closeChannels()
		// Delete subscription from book
		ws.OrderBookMgr.Mutex.Lock()
		if ws.OrderBookMgr.OrderBooks != nil {
			if _, ok := ws.OrderBookMgr.OrderBooks[channelName]; ok {
				delete(ws.OrderBookMgr.OrderBooks[channelName], pair)
			} else {
				ws.ErrorLogger.Printf("UnsubscribeBook error | channel name %s does not exist in OrderBooks\n", channelName)
			}
		} else {
			ws.ErrorLogger.Println("UnsubscribeBook error | OrderBooks is nil")
		}
		ws.OrderBookMgr.Mutex.Unlock()
	}
}

// Appends incoming replenishEntry to end of slice. Should only be used with
// "replenish" orders identified by 'UpdateType' field == "r".
func (ob *InternalOrderBook) replenishEntry(replenishEntry *InternalBookEntry, internalEntries *[]InternalBookEntry) error {
	*internalEntries = append(*internalEntries, *replenishEntry)
	return nil
}

// Performs binary search on slice pre-sorted ascending by Price field to find
// existing entry with same price level as incoming deleteEntry and deletes it.
// Should only be used with incoming deleteEntry with 'Volume' field == 0.
func (ob *InternalOrderBook) deleteAskEntry(deleteEntry *InternalBookEntry, internalEntries *[]InternalBookEntry) error {
	i := sort.Search(len(*internalEntries), func(i int) bool {
		return (*internalEntries)[i].Price.Cmp(deleteEntry.Price) >= 0
	})

	if i < len(*internalEntries) && (*internalEntries)[i].Price.Equal(deleteEntry.Price) {
		*internalEntries = append((*internalEntries)[:i], (*internalEntries)[i+1:]...)
	} else {
		return fmt.Errorf("deleteaskentry error | entry not found | \ndeleteEntry: %v\nCurrent Asks: %v", deleteEntry, *internalEntries)
	}
	return nil
}

// Performs binary search on slice pre-sorted descending by Price field to find
// existing entry with same price level as incoming deleteEntry and deletes it.
// Should only be used with incoming deleteEntry with 'Volume' field == 0.
func (ob *InternalOrderBook) deleteBidEntry(deleteEntry *InternalBookEntry, internalEntries *[]InternalBookEntry) error {
	i := sort.Search(len(*internalEntries), func(i int) bool {
		return (*internalEntries)[i].Price.Cmp(deleteEntry.Price) <= 0
	})

	if i < len(*internalEntries) && (*internalEntries)[i].Price.Equal(deleteEntry.Price) {
		*internalEntries = append((*internalEntries)[:i], (*internalEntries)[i+1:]...)
	} else {
		return fmt.Errorf("deletebidentry error | entry not found | \ndeleteEntry: %v\nCurrent Bids: %v", deleteEntry, *internalEntries)
	}
	return nil
}

// Performs binary search on slice pre-sorted ascending by Price field to find
// existing entry with same price level as incoming updateEntry. Updates volume
// if entry exists or inserts and cuts slice back down to size if it doesn't exist
func (ob *InternalOrderBook) updateAskEntry(updateEntry *InternalBookEntry, internalEntries *[]InternalBookEntry) error {
	// binary search to find index closest to updateEntry price
	i := sort.Search(len(*internalEntries), func(i int) bool {
		return (*internalEntries)[i].Price.Cmp(updateEntry.Price) >= 0
	})
	// if entry exists, update volume
	if i < len(*internalEntries) && (*internalEntries)[i].Price.Equal(updateEntry.Price) {
		(*internalEntries)[i].Volume = updateEntry.Volume
	} else {
		*internalEntries = append(*internalEntries, InternalBookEntry{})
		copy((*internalEntries)[i+1:], (*internalEntries)[i:])
		(*internalEntries)[i] = *updateEntry
		*internalEntries = (*internalEntries)[:len(*internalEntries)-1]
	}
	return nil
}

// Performs binary search on slice pre-sorted descending by Price field to find
// existing entry with same price level as incoming updateEntry. Updates volume
// if entry exists or inserts and cuts slice back down to size if it doesn't exist
func (ob *InternalOrderBook) updateBidEntry(updateEntry *InternalBookEntry, internalEntries *[]InternalBookEntry) error {
	// binary search to find index closest to updateEntry price
	i := sort.Search(len(*internalEntries), func(i int) bool {
		return (*internalEntries)[i].Price.Cmp(updateEntry.Price) <= 0
	})
	// if entry exists, update volume
	if i < len(*internalEntries) && (*internalEntries)[i].Price.Equal(updateEntry.Price) {
		(*internalEntries)[i].Volume = updateEntry.Volume
	} else {
		*internalEntries = append(*internalEntries, InternalBookEntry{})
		copy((*internalEntries)[i+1:], (*internalEntries)[i:])
		(*internalEntries)[i] = *updateEntry
		*internalEntries = (*internalEntries)[:len(*internalEntries)-1]
	}
	return nil
}

// Builds checksum string from top 10 asks and bids per the Kraken docs and
// validates it against incoming checksum message 'msgChecksum'. Returns an error
// if they do not match.
func (ob *InternalOrderBook) validateChecksum(msgChecksum uint32) error {
	var buffer bytes.Buffer
	buffer.Grow(260)
	ob.Mutex.RLock()
	// write to buffer from asks
	for i := 0; i < 10; i++ {
		price := strings.TrimLeft(strings.ReplaceAll(ob.Asks[i].Price.StringFixed(5), ".", ""), "0")
		buffer.WriteString(price)
		volume := strings.TrimLeft(strings.ReplaceAll(ob.Asks[i].Volume.StringFixed(8), ".", ""), "0")
		buffer.WriteString(volume)
	}

	// write to buffer from bids
	for i := 0; i < 10; i++ {
		price := strings.TrimLeft(strings.ReplaceAll(ob.Bids[i].Price.StringFixed(5), ".", ""), "0")
		buffer.WriteString(price)
		volume := strings.TrimLeft(strings.ReplaceAll(ob.Bids[i].Volume.StringFixed(8), ".", ""), "0")
		buffer.WriteString(volume)
	}
	ob.Mutex.RUnlock()
	if checksum := crc32.ChecksumIEEE(buffer.Bytes()); checksum != msgChecksum {
		return fmt.Errorf("invalid checksum")
	}
	return nil
}

// Completes unsubscribing by sending a message to ob.DoneChan.
func (ob *InternalOrderBook) unsubscribe() {
	ob.DoneChan <- struct{}{}
}

// Safely closes the DataChan and DoneChan with atomic flags set before closing.
func (ob *InternalOrderBook) closeChannels() {
	atomic.StoreInt32(&ob.DataChanClosed, 1)
	close(ob.DataChan)
	atomic.StoreInt32(&ob.DoneChanClosed, 1)
	close(ob.DoneChan)
}

// Builds initial state of book from first message (arg 'msg') after subscribing
// to new book channel.
func (ob *InternalOrderBook) buildInitialBook(msg *WSOrderBookSnapshot) error {
	ob.Mutex.Lock()
	defer ob.Mutex.Unlock()
	capacity := len(msg.Bids)
	ob.Bids = make([]InternalBookEntry, capacity)
	ob.Asks = make([]InternalBookEntry, capacity)
	for i, bid := range msg.Bids {
		newBid, err := stringEntryToDecimal(&bid)
		if err != nil {
			return fmt.Errorf("error calling stringentrytodecimal | %w", err)
		}
		ob.Bids[i] = *newBid
	}
	for i, ask := range msg.Asks {
		newAsk, err := stringEntryToDecimal(&ask)
		if err != nil {
			return fmt.Errorf("error calling stringentrytodecimal | %w", err)
		}
		ob.Asks[i] = *newAsk
	}
	return nil
}

// #endregion

// #region Helper functions

// Converts Price Volume and Time string fields from WSBookEntry to decimal.decimal
// type, initializes InternalBookEntry type with the decimal.decimal values and
// returns it.
func stringEntryToDecimal(entry *WSBookEntry) (*InternalBookEntry, error) {
	decimalPrice, err := decimal.NewFromString(entry.Price)
	if err != nil {
		return nil, err
	}
	decimalVolume, err := decimal.NewFromString(entry.Volume)
	if err != nil {
		return nil, err
	}
	decimalTime, err := decimal.NewFromString(entry.Time)
	if err != nil {
		return nil, err
	}
	return &InternalBookEntry{
		Price:  decimalPrice,
		Volume: decimalVolume,
		Time:   decimalTime,
	}, nil
}

func buildReqIdBuffer(options []ReqIDOption) (bytes.Buffer, error) {
	var buffer bytes.Buffer
	if len(options) > 0 {
		if len(options) > 1 {
			return bytes.Buffer{}, ErrTooManyArgs
		}
		for _, option := range options {
			option(&buffer)
		}
		return buffer, nil
	}
	return bytes.Buffer{}, nil
}

// #endregion
