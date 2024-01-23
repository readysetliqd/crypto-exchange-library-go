package krakenspot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

// TODO write a connect public only method
// TODO write a connect private only method
// TODO move all the public methods to its own file or rename this one
// TODO add order response callback initializer method and add callback to ws manager struct
// TODO add optional reqid to ALL websocket requests
// TODO write a SetLogger method and add Logger to WebSocketManager struct for error handling
// TODO add an initializer method for orderStatusCallback
// TODO add OrderManager to keep current state of open trades in memory
// TODO add order writer to write all orderIDs opened during program operation in case disconnect
// TODO add trades logger and initializer to write trades to file
// TODO test if callbacks can be nil and end user can access channels with go routines

// #region Exported WebSocket connection methods (Connect, Subscribe<>, and Unsubscribe<>)

// TODO update docstrings after connect public and private only methods are written
// Creates both authenticated and public connections to Kraken WebSocket server.
// Use ConnectPublic() instead if private channels aren't needed. Accepts arg
// 'systemStatusCallback' callback function which an end user can implement
// their own logic on handling incoming system status change messages.
//
// Note: Creates authenticated token with AuthenticateWebSockets() method which
// expires within 15 minutes. Strongly recommended to subscribe to at least one
// private WebSocket channel and leave it open.
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
	if systemStatusCallback == nil {
		log.Println(`
		WARNING: Passing nil to arg 'systemStatusCallback' may 
		result in program crashes or invalid messages being pushed to Kraken's 
		server on the occasions where their system's status is changed. Ensure 
		you have implemented handling system status changes on your own or 
		reinitialize the client with a valid systemStatusCallback function
		`)
	}
	err := kc.AuthenticateWebSockets()
	if err != nil {
		// TODO implement retry, 403 error when system is under maintenance,
		// GetSystemStatus also returns 403 when under maintenance. does it return
		// 403 for prohibited vpn locations? no internet err contains "no such host"
		return fmt.Errorf("error authenticating websockets | %w", err)
	}
	err = kc.dialKraken(wsPrivateURL)
	if err != nil {
		return fmt.Errorf("error dialing kraken private url | %w", err)
	}
	err = kc.dialKraken(wsPublicURL)
	if err != nil {
		return fmt.Errorf("error dialing kraken public url | %w", err)
	}
	kc.startMessageReader()
	kc.startAuthMessageReader()
	kc.WebSocketManager.Mutex.Lock()
	kc.WebSocketManager.SystemStatusCallback = systemStatusCallback
	kc.WebSocketManager.Mutex.Unlock()
	return nil
}

// Iterates through all open public and private subscriptions and sends an
// unsubscribe message to Kraken's WebSocket server for each.
//
// # Example Usage:
//
//	err := kc.UnsubscribeAll()
func (ws *WebSocketManager) UnsubscribeAll() error {
	ws.SubscriptionMgr.Mutex.Lock()
	for channelName := range ws.SubscriptionMgr.PrivateSubscriptions {
		switch channelName {
		case "ownTrades":
			err := ws.UnsubscribeOwnTrades()
			if err != nil {
				return fmt.Errorf("error unsubscribing from owntrades | %w", err)
			}
		case "openOrders":
			err := ws.UnsubscribeOpenOrders()
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
				ws.UnsubscribeTicker(pair)
			}
		case channelName == "trade":
			for pair := range pairMap {
				ws.UnsubscribeTrade(pair)
			}
		case channelName == "spread":
			for pair := range pairMap {
				ws.UnsubscribeSpread(pair)
			}
		case strings.HasPrefix(channelName, "ohlc"):
			_, intervalStr, _ := strings.Cut(channelName, "-")
			interval, err := strconv.ParseUint(intervalStr, 10, 16)
			if err != nil {
				return fmt.Errorf("error parsing uint from interval | %w", err)
			}
			for pair := range pairMap {
				ws.UnsubscribeOHLC(pair, uint16(interval))
			}
		case strings.HasPrefix(channelName, "book"):
			_, depthStr, _ := strings.Cut(channelName, "-")
			depth, err := strconv.ParseUint(depthStr, 10, 16)
			if err != nil {
				return fmt.Errorf("error parsing uint from depth | %w", err)
			}
			for pair := range pairMap {
				ws.UnsubscribeBook(pair, uint16(depth))
			}
		}
	}
	return nil
}

// Subscribes to "ticker" WebSocket channel for arg 'pair'. Must pass a valid
// callback function to dictate what to do with incoming data.
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
func (ws *WebSocketManager) SubscribeTicker(pair string, callback GenericCallback) error {
	channelName := "ticker"
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
	err := ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "ticker" WebSocket channel for arg 'pair'
//
// # Example Usage:
//
//	err := kc.UnsubscribeTicker("XBT/USD")
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeTicker(pair string) error {
	channelName := "ticker"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
	err := ws.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing message | %w", err)
		return err
	}
	return nil
}

// Subscribes to "ohlc" WebSocket channel for arg 'pair' and specified 'interval'
// in minutes. On subscribing, sends last valid closed candle (had at least one
// trade), irrespective of time. Must pass a valid callback function to dictate
// what to do with incoming data.
//
// # Enum:
//
// 'interval' - 1, 5, 15, 30, 60, 240, 1440, 10080, 21600
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
func (ws *WebSocketManager) SubscribeOHLC(pair string, interval uint16, callback GenericCallback) error {
	name := "ohlc"
	channelName := fmt.Sprintf("%s-%v", name, interval)
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s", "interval": %v}}`, pair, name, interval)
	err := ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "ohlc" WebSocket channel for arg 'pair' and specified 'interval'
// in minutes.
//
// # Enum:
//
// 'interval' - 1, 5, 15, 30, 60, 240, 1440, 10080, 21600
//
// # Example Usage:
//
//	err := kc.UnsubscribeOHLC("XBT/USD", 1)
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeOHLC(pair string, interval uint16) error {
	channelName := "ohlc"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s", "interval": %v}}`, pair, channelName, interval)
	err := ws.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing message | %w", err)
		return err
	}
	return nil
}

// Subscribes to "trade" WebSocket channel for arg 'pair'. Must pass a valid
// callback function to dictate what to do with incoming data.
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
func (ws *WebSocketManager) SubscribeTrade(pair string, callback GenericCallback) error {
	channelName := "trade"
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
	err := ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "trade" WebSocket channel for arg 'pair'
//
// # Example Usage:
//
//	err := kc.UnsubscribeTrade("XBT/USD")
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeTrade(pair string) error {
	channelName := "trade"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
	err := ws.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing message | %w", err)
		return err
	}
	return nil
}

// Subscribes to "spread" WebSocket channel for arg 'pair'. Must pass a valid
// callback function to dictate what to do with incoming data.
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
func (ws *WebSocketManager) SubscribeSpread(pair string, callback GenericCallback) error {
	channelName := "spread"
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
	err := ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "spread" WebSocket channel for arg 'pair'
//
// # Example Usage:
//
//	err := kc.UnsubscribeSpread("XBT/USD")
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeSpread(pair string) error {
	channelName := "spread"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
	err := ws.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing message | %w", err)
		return err
	}
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
// When nil is passed, access state of the orderbook with the following methods:
//
//	func (ws *WebSocketManager) GetBookState(pair string, depth uint16) (BookState, error)
//
//	func (ws *WebSocketManager) ListAsks(pair string, depth uint16) ([]InternalBookEntry, error)
//
//	func (ws *WebSocketManager) ListBids(pair string, depth uint16) ([]InternalBookEntry, error)
//
// # Enum:
//
// 'depth' - 10, 25, 100, 500, 1000
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
func (ws *WebSocketManager) SubscribeBook(pair string, depth uint16, callback GenericCallback) error {
	name := "book"
	channelName := fmt.Sprintf("%s-%v", name, depth)
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s", "depth": %v}}`, pair, name, depth)
	if callback == nil {
		callback = ws.bookCallback(channelName, pair, depth)
	}
	err := ws.subscribePublic(channelName, payload, pair, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribepublic method | %w", err)
	}
	return nil
}

// Unsubscribes from "book" WebSocket channel for arg 'pair' and specified 'depth'
// as number of book entries for each bids and asks.
//
// # Enum:
//
// 'depth' - 10, 25, 100, 500, 1000
//
// # Example Usage:
//
//	err := kc.UnsubscribeBook("XBT/USD", 10)
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeBook(pair string, depth uint16) error {
	channelName := "book"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s", "depth": %v}}`, pair, channelName, depth)
	err := ws.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing message | %w", err)
		return err
	}
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

// Subscribes to "ownTrades" authenticated WebSocket channel. Must pass a valid
// callback function to dictate what to do with incoming data. Accepts none or
// many functional options args 'options'.
//
// # Functional Options:
//
//	// Whether to consolidate order fills by root taker trade(s). If false, all order fills will show separately. Defaults to true if not called.
//	func WithoutConsolidatedTaker()
//	// Whether to send historical feed data snapshot upon subscription. Defaults to true if not called.
//	func WithoutSnapshot()
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
	var buffer bytes.Buffer
	for _, option := range options {
		option(&buffer)
	}
	channelName := "ownTrades"
	payload := fmt.Sprintf(`{"event": "subscribe", "subscription": {"name": "%s", "token": "%s"%s}}`, channelName, ws.WebSocketToken, buffer.String())
	err := ws.subscribePrivate(channelName, payload, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribeprivate method | %w", err)
	}
	return nil
}

// Unsubscribes from "ownTrades" WebSocket channel.
//
// # Example Usage:
//
//	err := kc.UnsubscribeOwnTrades()
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeOwnTrades() error {
	channelName := "ownTrades"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "subscription": {"name": "%s", "token": "%s"}}`, channelName, ws.WebSocketToken)
	err := ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		return fmt.Errorf("error writing message to auth client | %w", err)
	}
	return nil
}

// Subscribes to "openOrders" authenticated WebSocket channel. Must pass a valid
// callback function to dictate what to do with incoming data. Accepts none or
// one functional options arg passed to 'options'.
//
// # Functional Options:
//
//	// Whether to send rate-limit counter in updates  Defaults to false if not called.
//	func WithRateCounter()
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
	var buffer bytes.Buffer
	for _, option := range options {
		option(&buffer)
	}
	channelName := "openOrders"
	payload := fmt.Sprintf(`{"event": "subscribe", "subscription": {"name": "%s", "token": "%s"%s}}`, channelName, ws.WebSocketToken, buffer.String())
	err := ws.subscribePrivate(channelName, payload, callback)
	if err != nil {
		return fmt.Errorf("error calling subscribeprivate method | %w", err)
	}
	return nil
}

// Unsubscribes from "openOrders" authenticated WebSocket channel.
//
// # Example Usage:
//
//	err := kc.UnsubscribeOpenOrders()
//	if err != nil...
func (ws *WebSocketManager) UnsubscribeOpenOrders() error {
	channelName := "openOrders"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "subscription": {"name": "%s", "token": "%s"}}`, channelName, ws.WebSocketToken)
	err := ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		return fmt.Errorf("error writing message to auth client | %w", err)
	}
	return nil
}

// #endregion

// #region Exported *WebSocketManager Order methods (addOrder, editOrder, cancelOrder(s))

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
	err := ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		return fmt.Errorf("error writing message to auth client | %w", err)
	}
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
	err := ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		return fmt.Errorf("error writing message to auth client | %w", err)
	}
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
	err := ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		return fmt.Errorf("error writing message to auth client | %w", err)
	}
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
	err = ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		return fmt.Errorf("error writing message to auth client | %w", err)
	}
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
	err := ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		return fmt.Errorf("error writing message to auth client | %w", err)
	}
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
	err := ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		return fmt.Errorf("error writing message to auth client | %w", err)
	}
	return nil
}

// #endregion

// #region *WebSocketManager helper methods (subscribe, readers, routers, and connections)

// Helper method for public data subscribe methods to handle initializing
// Subscription, sending payload to server, and starting go routine with
// channels to listen for incoming messages.
func (ws *WebSocketManager) subscribePublic(channelName, payload, pair string, callback GenericCallback) error {
	if callback == nil {
		return fmt.Errorf("callback function must not be nil")
	}
	if !publicChannelNames[channelName] {
		return fmt.Errorf("unknown channel name; check valid depth or interval against enum | %s", channelName)
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
	err := ws.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing subscription message | %w", err)
		return err
	}

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
	if callback == nil {
		return fmt.Errorf("callback function must not be nil")
	}
	if !privateChannelNames[channelName] {
		return fmt.Errorf("unknown channel name; check valid depth or interval against enum | %s", channelName)
	}

	sub := newSub(channelName, "", callback)

	// Assign Subscription to map
	ws.SubscriptionMgr.Mutex.Lock()
	ws.SubscriptionMgr.PrivateSubscriptions[channelName] = sub
	ws.SubscriptionMgr.Mutex.Unlock()

	// Build payload and send subscription message
	err := ws.AuthWebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing subscription message | %w", err)
		return err
	}

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
					delete(ws.SubscriptionMgr.PrivateSubscriptions, channelName)
					ws.SubscriptionMgr.Mutex.Unlock()
					return
				}
			}
		}
	}()
	return nil
}

// Starts a goroutine that continuously reads messages from the WebSocket
// connection. If the message is not a heartbeat message, it routes the message.
func (ws *WebSocketManager) startMessageReader() {
	go func() {
		for {
			_, msg, err := ws.WebSocketClient.ReadMessage()
			if err != nil {
				log.Println("error reading message | ", err)
				// TODO figure out reconnect logic, reconnect here? route error to somewhere else?
				// TODO this error occurs if connect, subscribe, unsubscribe, subscribe, unsubscribe
				// TODO from AUTHENTICATED. but somehow the private reader threw this error?
				// if strings.Contains(err.Error(), "close 1006") { // abnormal closure: unexpected EOF
				// }
				continue
			}
			if !bytes.Equal(heartbeat, msg) { // not a heartbeat message
				err := ws.routeMessage(msg)
				if err != nil {
					log.Println(err)
				}
			} else { //DEBUG
				continue
			}
		}
	}()
}

// Starts a goroutine that continuously reads messages from the private WebSocket
// connection. If the message is not a heartbeat message, it routes the message.
func (ws *WebSocketManager) startAuthMessageReader() {
	go func() {
		for {
			_, msg, err := ws.AuthWebSocketClient.ReadMessage()
			if err != nil {
				log.Println("error reading message | ", err)
				continue
			}
			if !bytes.Equal(heartbeat, msg) { // not a heartbeat message
				err := ws.routeMessage(msg)
				if err != nil {
					log.Println(err)
				}
			} else { //DEBUG
				continue
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
		log.Println("pong | reqid: ", v.ReqID)
	case WSErrorResp:
		return fmt.Errorf("error message: %s | reqid: %d", v.ErrorMessage, v.ReqID)
	default:
		return fmt.Errorf("cannot route unknown msg type | %s", msg.Event)
	}
	return nil
}

// // TODO implement maintain connection logic, should be some sort of timer,
// // probably an open channel that gets a message every time an incoming message
// // is received/processed by routeMessage()
// func (kc *KrakenClient) maintainConnection() {
// 	// Set a read deadline for the connection
// 	kc.WebSocketClient.SetReadDeadline(time.Now().Add(wsTimeoutDuration))

// 	for {
// 		_, _, err := kc.WebSocketClient.ReadMessage()
// 		if err != nil {
// 			// Check if the error is due to a timeout
// 			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
// 				// No messages received within the timeout duration, reconnect
// 				kc.reconnect()
// 			} else {
// 				log.Printf("Error reading WebSocket message | %v", err)
// 				break
// 			}
// 			continue
// 		}

// 		// Reset the read deadline upon receiving a message
// 		kc.WebSocketClient.SetReadDeadline(time.Now().Add(wsTimeoutDuration))
// 	}
// }

// // TODO implement reconnection logic recursive?
// func (kc *KrakenClient) reconnect() error {
// 	kc.WebSocketMutex.Lock()
// 	kc.WebSocketClient = nil
// 	kc.WebSocketMutex.Unlock()
// 	err := kc.dialKraken()
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (ws *WebSocketManager) dialKraken(url string) error {
	ws.Mutex.Lock()
	defer ws.Mutex.Unlock()
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{})

	if err != nil {
		err = fmt.Errorf("error dialing kraken | %w", err)
		return err
	}
	switch url {
	case wsPublicURL:
		ws.WebSocketClient = conn
	case wsPrivateURL:
		ws.AuthWebSocketClient = conn
	}
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
		DataChan:            make(chan interface{}),
		DoneChan:            make(chan struct{}),
		ConfirmedChan:       make(chan struct{}),
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
					log.Printf("error building initial state of book; sending unsubscribe msg; try subscribing again | %s", err)
					err := ws.UnsubscribeBook(pair, depth)
					if err != nil {
						log.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s", err)
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
											log.Printf("error calling stringEntryToDecimal; stopping goroutine and unsubscribing | %s", err)
											err := ws.UnsubscribeBook(pair, depth)
											if err != nil {
												log.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s", err)
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
											log.Printf("error calling replenish method; stopping goroutine and unsubscribing | %s", err)
											err := ws.UnsubscribeBook(pair, depth)
											if err != nil {
												log.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s", err)
											}
										}
									}
								}
								if len(bookUpdate.Bids) > 0 {
									for _, bid := range bookUpdate.Bids {
										newEntry, err := stringEntryToDecimal(&bid)
										if err != nil {
											log.Printf("error calling stringEntryToDecimal; stopping goroutine and unsubscribing | %s", err)
											err := ws.UnsubscribeBook(pair, depth)
											if err != nil {
												log.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s", err)
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
											log.Printf("error calling replenish/update/delete method; stopping goroutine and unsubscribing | %s", err)
											err := ws.UnsubscribeBook(pair, depth)
											if err != nil {
												log.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s", err)
											}
										}
									}
								}
								ob.Mutex.Unlock()
								// Stop go routine and unsubscribe if checksum does not pass
								checksum, err := strconv.ParseUint(bookUpdate.Checksum, 10, 32)
								if err != nil {
									log.Printf("error parsing checksum uint32; stopping goroutine and unsubscribing | %s", err)
									err := ws.UnsubscribeBook(pair, depth)
									if err != nil {
										log.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s", err)
									}
									return
								}
								if err := ob.validateChecksum(uint32(checksum)); err != nil {
									log.Printf("error validating checksum; stopping goroutine and unsubscribing | %s", err)
									err := ws.UnsubscribeBook(pair, depth)
									if err != nil {
										log.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s", err)
									}
									return
								}
							}
						case <-ob.DoneChan:
							if ob.DoneChanClosed == 0 {
								ob.closeChannels()
								// Delete subscription from book
								ws.OrderBookMgr.Mutex.Lock()
								if ws.OrderBookMgr.OrderBooks != nil {
									if _, ok := ws.OrderBookMgr.OrderBooks[channelName]; ok {
										delete(ws.OrderBookMgr.OrderBooks[channelName], pair)
									} else {
										log.Printf("UnsubscribeBook error | channel name %s does not exist in OrderBooks", channelName)
									}
								} else {
									log.Println("UnsubscribeBook error | OrderBooks is nil")
								}
								ws.OrderBookMgr.Mutex.Unlock()
							}
							return
						}
					}
				}()
			} else {
				log.Println("unknown data type sent to book callback")
			}
		}
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

// #endregion
