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

// #region Exported *KrakenClient and *WebSocketManager methods (Connect Subscribe<> and Unsubscribe<>)

// Creates authenticated connection to Kraken WebSocket server. Accepts arg
// 'systemStatusCallback' callback function which an end user can implement
// their own logic on handling incoming system status change messages.
//
// Note: Creates authenticated token which expires within 15 minutes. If
// private-data channel subscription is desired, recommened subscribing to at
// least one private-data channel before token expiry (ownTrades or openOrders)
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
		return fmt.Errorf("error authenticating websockets | %w", err)
	}
	err = kc.dialKraken()
	if err != nil {
		return fmt.Errorf("error dialing kraken | %w", err)
	}
	kc.startMessageReader()
	kc.WebSocketManager.Mutex.Lock()
	kc.WebSocketManager.SystemStatusCallback = systemStatusCallback
	kc.WebSocketManager.Mutex.Unlock()
	return nil
}

// // TODO finish implementation
// // TODO test
// // TODO write docstrings
// func (ws *WebSocketManager) UnsubscribeAll() error {
// 	ws.SubscriptionMgr.Mutex.Lock()
// 	for channelName := range ws.SubscriptionMgr.PrivateSubscriptions {
// 		switch channelName {
// 		case "ownTrades":
// 			err := ws.UnsubscribeOwnTrades(ws.WebSocketToken)
// 			if err != nil {
// 				return fmt.Errorf("error unsubscribing from owntrades | %w", err)
// 			}
// 		case "openOrders":
// 			err := ws.UnsubscribeOpenOrders(ws.WebSocketToken)
// 			if err != nil {
// 				return fmt.Errorf("error unsubscribing from openorders | %w", err)
// 			}
// 		default:
// 			return fmt.Errorf("unknown channel name %s", channelName)
// 		}
// 	}
// 	for channelName, pairMap := range ws.SubscriptionMgr.PublicSubscriptions {
// 		switch {
// 		case channelName == "ticker":
// 			for pair := range pairMap {
// 				ws.UnsubscribeTicker(pair)
// 			}
// 		case channelName == "trade":
// 			for pair := range pairMap {
// 				ws.UnsubscribeTrade(pair)
// 			}
// 		case channelName == "spread":
// 			for pair := range pairMap {
// 				ws.UnsubscribeSpread(pair)
// 			}
// 		case strings.HasPrefix(channelName, "ohlc"):
// 			_, intervalStr, _ := strings.Cut(channelName, "-")
// 			interval, err := strconv.ParseUint(intervalStr, 10, 16)
// 			if err != nil {
// 				return fmt.Errorf("error parsing uint from interval | %w", err)
// 			}
// 			for pair := range pairMap {
// 				ws.UnsubscribeOHLC(pair, uint16(interval))
// 			}
// 		case strings.HasPrefix(channelName, "book"):
// 			_, depthStr, _ := strings.Cut(channelName, "-")
// 			depth, err := strconv.ParseUint(depthStr, 10, 16)
// 			if err != nil {
// 				return fmt.Errorf("error parsing uint from depth | %w", err)
// 			}
// 			for pair := range pairMap {
// 				ws.UnsubscribeBook(pair, uint16(depth))
// 			}
// 		}
// 	}
// 	return nil
// }

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
//	func initialStateMsg(msg krakenspot.WSOrderBook) bool {
//		//...your implementation here
//	}
//	func initializeBook(msg krakenspot.WSOrderBook, book *Book) {
//		//...your implementation here
//	}
//	func updateBook(msg krakenspot.WSOrderBook, book *Book) {
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

// Starts a goroutine that continuously reads messages from the WebSocket
// connection. If the message is not a heartbeat message, it routes the message.
func (ws *WebSocketManager) startMessageReader() {
	go func() {
		for {
			_, msg, err := ws.WebSocketClient.ReadMessage()
			if err != nil {
				log.Println("error reading message | ", err)
				continue
			}
			if !bytes.Equal(heartbeat, msg) { // not a heartbeat message
				err := ws.routeMessage(msg)
				if err != nil {
					log.Println(err)
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
	case WSBookResp:
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
	//TODO implement this
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routeOrderMessage(msg *GenericMessage) error {
	//TODO implement this
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routeGeneralMessage(msg *GenericMessage) error {
	switch msg.Event {
	case "subscriptionStatus":
		if subscriptionStatusMsg, ok := msg.Content.(WSSubscriptionStatus); !ok {
			return fmt.Errorf("error asserting msg.content to wssubscriptionstatus type")
		} else {
			switch subscriptionStatusMsg.Status {
			case "subscribed":
				if publicChannelNames[subscriptionStatusMsg.ChannelName] {
					ws.SubscriptionMgr.PublicSubscriptions[subscriptionStatusMsg.ChannelName][subscriptionStatusMsg.Pair].confirmSubscription()
				} else if privateChannelNames[subscriptionStatusMsg.ChannelName] {
					ws.SubscriptionMgr.PrivateSubscriptions[subscriptionStatusMsg.ChannelName].confirmSubscription()
				}
			case "unsubscribed":
				if publicChannelNames[subscriptionStatusMsg.ChannelName] {
					ws.SubscriptionMgr.PublicSubscriptions[subscriptionStatusMsg.ChannelName][subscriptionStatusMsg.Pair].unsubscribe()
					if strings.HasPrefix(subscriptionStatusMsg.ChannelName, "book") {
						ws.OrderBookMgr.OrderBooks[subscriptionStatusMsg.ChannelName][subscriptionStatusMsg.Pair].unsubscribe()
					}
				} else if privateChannelNames[subscriptionStatusMsg.ChannelName] {
					ws.SubscriptionMgr.PrivateSubscriptions[subscriptionStatusMsg.ChannelName].unsubscribe()
				}
			case "error":
				return fmt.Errorf("subscribe/unsubscribe error msg received. operation not completed; check inputs and try again | %s", subscriptionStatusMsg.ErrorMessage)
			default:
				return fmt.Errorf("cannot route unknown subscriptionStatus status | %s", subscriptionStatusMsg.Status)
			}
		}
	case "systemStatus":
		if systemStatusMsg, ok := msg.Content.(WSSystemStatus); !ok {
			return fmt.Errorf("error asserting msg.content to wssystemstatus type")
		} else {
			if ws.SystemStatusCallback != nil {
				ws.SystemStatusCallback(systemStatusMsg.Status)
			} else {
				log.Printf("system status: %s", systemStatusMsg.Status)
			}
		}
	case "pong":
		if pongMsg, ok := msg.Content.(WSPong); !ok {
			return fmt.Errorf("error asserting msg.content to wspong type")
		} else {
			log.Println(pongMsg.Event, pongMsg.ReqID)
		}
	default:
		return fmt.Errorf("cannot route unknown event type | %s", msg.Event)
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

func (ws *WebSocketManager) dialKraken() error {
	ws.Mutex.Lock()
	defer ws.Mutex.Unlock()
	conn, _, err := websocket.DefaultDialer.Dial(wsPublicURL, http.Header{})

	if err != nil {
		err = fmt.Errorf("error dialing kraken | %w", err)
		return err
	}
	ws.WebSocketClient = conn
	// var initResponse WSSystemStatus
	// err = ws.WebSocketClient.ReadJSON(&initResponse)
	// if err != nil {
	// 	err = fmt.Errorf("error reading json | %w", err)
	// 	return err
	// }
	// if !(initResponse.Event == "systemStatus" && initResponse.Status == "online") {
	// 	err = fmt.Errorf("error establishing websockets connection. system status | %s", initResponse.Status)
	// 	return err
	// }
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
		ChannelName:    channelName,
		Pair:           pair,
		Callback:       callback,
		DataChan:       make(chan interface{}),
		DoneChan:       make(chan struct{}),
		ConfirmedChan:  make(chan struct{}),
		DataChanClosed: 0,
		DoneChanClosed: 0,
	}
}

// #endregion

// #region *InternalOrderBook helper methods (bookCallback, unsubscribe, close, buildInitial, checksum, update)

func (ws *WebSocketManager) bookCallback(channelName, pair string, depth uint16) func(data interface{}) {
	return func(data interface{}) {
		if msg, ok := data.(WSBookResp); ok {
			if _, ok := ws.OrderBookMgr.OrderBooks[channelName]; !ok {
				ws.OrderBookMgr.Mutex.Lock()
				ws.OrderBookMgr.OrderBooks[channelName] = make(map[string]*InternalOrderBook)
				ws.OrderBookMgr.Mutex.Unlock()
			}
			if _, ok := ws.OrderBookMgr.OrderBooks[channelName][pair]; !ok {
				ws.OrderBookMgr.Mutex.Lock()
				ws.OrderBookMgr.OrderBooks[channelName][pair] = &InternalOrderBook{
					DataChan:       make(chan WSOrderBook),
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
				ws.OrderBookMgr.OrderBooks[channelName][pair].DataChan <- msg.OrderBook
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
func (ob *InternalOrderBook) buildInitialBook(msg *WSOrderBook) error {
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
