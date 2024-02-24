// Package krakenspot is a comprehensive toolkit for interfacing with the Kraken
// Spot Exchange API. It enables WebSocket and REST API interactions, including
// subscription to both public and private channels. The package provides a
// client for initiating these interactions and a state manager for handling
// them.
//
// The websocket.go file specifically handles all interactions with Kraken's
// WebSocket server. This includes connecting, subscribing to public and
// private channels and sending orders among other operations. The code in
// this file is organized in sections, reflecting the order in which they would
// typically be called by an end user importing this package.
package krakenspot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

// #region Exported methods for *WebSocketManager Start/Stop<Feature>  (OrderBookManager, TradeLogger, OpenOrderManager, TradingRateLimiter)

// StartOrderBookManager changes the flag for ws.OrderBookMgr to signal new
// SubscribeBook() subscription calls to track the current state of book in memory.
// Does not apply to already existing subscriptions, unsubscribe and resubscribe
// as necessary.
func (ws *WebSocketManager) StartOrderBookManager() error {
	if !ws.OrderBookMgr.isTracking.CompareAndSwap(false, true) {
		return fmt.Errorf("OrderBookManager already running")
	}
	return nil
}

// StopOrderBookManager changes the flag for ws.OrderBookMgr to signal new
// SubscribeBook() subscription calls not to track the current state of book
// in memory. Does not apply to already existing subscriptions, unsubscribe
// and resubscribe as necessary.
func (ws *WebSocketManager) StopOrderBookManager() error {
	if !ws.OrderBookMgr.isTracking.CompareAndSwap(true, false) {
		return fmt.Errorf("OrderBookManager already stopped")
	}
	return nil
}

// StartTradeLogger() starts a trade logger which opens, or creates if not exists,
// a file with 'filename' and writes all incoming trades from an "ownTrades"
// WebSocket channel subscription in json lines format. Suggested to call this
// method before SubscribeOwnTrades(). If errors occur, will log the errors
// to ErrorLogger (defaults to stdout if none is set) also structures the error
// message and logs it to the TradeLogger file.
//
// Note: Ignores "snapshot" trades
//
// # Example Usage:
//
//	err := kc.StartTradeLogger("todays_trades.jsonl")
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
	ctx, cancel := context.WithCancel(context.Background())
	ws.Mutex.Lock()
	ws.TradeLogger = &TradeLogger{
		file:       file,
		writer:     bufio.NewWriter(file),
		ch:         make(chan map[string]WSOwnTrade, 100),
		seqErrorCh: make(chan error),
		seq:        0,
		startSeq:   1,
		ctx:        ctx,
		cancel:     cancel,
	}
	ws.TradeLogger.isLogging.Store(true)
	ws.Mutex.Unlock()
	ws.TradeLogger.wg.Add(1)

	go func(ctx context.Context) {
		defer ws.TradeLogger.wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case trade, ok := <-ws.TradeLogger.ch:
				if !ok {
					return
				}
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
			case err, ok := <-ws.TradeLogger.seqErrorCh:
				if !ok {
					return
				}
				ws.handleTradeLoggerError("sequence error", err, nil)
			}
		}
	}(ctx)

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
		ws.TradeLogger.cancel()
		close(ws.TradeLogger.ch)
		close(ws.TradeLogger.seqErrorCh)
		ws.TradeLogger.wg.Wait()
		return ws.TradeLogger.file.Close()
	}
	return fmt.Errorf("trade logger is already stopped")
}

// Starts the open orders manager. Call this method before SubscribeOpenOrders().
// It will build initial state of all currently open orders and maintain it in
// memory as new open order update messages come in.
//
// # Example Usage:
//
//	// Print current state of open orders every 15 seconds
//	err := kc.StartOpenOrderManager()
//	if err != nil...
//	err := kc.SubscribeOpenOrders()
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

// StopOpenOrderManager() stops the internal tracking of current open orders.
// Closes channel and clears the internal OpenOrders map.
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

// StartTradingRateLimiter starts self rate-limiting for "order" type WebSocket
// methods. The "order" type methods include WSAddOrder(), WSEditOrder(),
// WSCancelOrder(), and WSCancelOrders(). Must have an active subscription to
// "openOrders" channel with SubscribeOpenOrders() before sending any orders
// for self rate-limiting logic to work correctly.
//
// Note: It's recommended to start open orders internal management with
// StartOpenOrderManager() before subscribing to "openOrders" channel. If open
// order manager is not started before subscribing, rate-limiter for edit
// orders will default to assuming a max incremement penalty and cancel orders
// will simply skip rate-limiting logic. Accepts one or none optional arg
// passed to 'maxCounterBufferPercent' which is a percent of the total max
// counter for your verification tier that will act as a buffer to attempt to
// avoid exceeding max counter rate when multiple order methods for the same
// pair are called consecutively and race to pass the rate-limit check.
//
// Note: Activating self rate-limiting may add significant processing overhead
// and waits and only attempts to prevent hitting the max. It is likely better to
// implement rate-limiting yourself and/or design your strategy not to approach
// rate-limits.
//
// # Enum:
//
// 'maxCounterBufferPercent' - [0..99]
//
// # Example Usage:
//
// Note: error assignment and handling omitted throughout
//
//	// Creates new client, connects, starts required features for rate-limiting
//	// with a 20% buffer, subscribes, and sends an order
//	kc := NewKrakenClient(apikey, apisecret, 2)
//	kc.ConnectPrivate()
//	kc.StartOpenOrderManager()
//	kc.StartTradingRateLimiter(20)
//	kc.SubscribeOpenOrders(openOrdersCallback)
//	// send orders as needed
//	kc.WSAddOrder(krakenspot.WSLimit("42100.20"), "buy", "1.0", "XBT/USD", krakenspot.WSPostOnly(), krakenspot.WSCloseLimit("44000"))
//	// etc...
func (ws *WebSocketManager) StartTradingRateLimiter(maxCounterBufferPercent ...uint8) error {
	if len(maxCounterBufferPercent) > 1 {
		return fmt.Errorf("too many args passed, expected 0 or 1")
	}
	if len(maxCounterBufferPercent) == 1 {
		if maxCounterBufferPercent[0] > 99 {
			return fmt.Errorf("invalid maxCounterBufferPercent")
		}
		ws.TradingRateLimiter.MaxCounter = ws.TradingRateLimiter.UnbufferedMaxCounter * (100 - int32(maxCounterBufferPercent[0])) / 100
	}
	if !ws.TradingRateLimiter.HandleRateLimit.CompareAndSwap(false, true) {
		return fmt.Errorf("trading rate-limiter was already initialized")
	}

	return nil
}

// StopTradingRateLimiter stops self rate-limiting for "order" type WebSocket
// methods.
func (ws *WebSocketManager) StopTradingRateLimiter() error {
	if !ws.TradingRateLimiter.HandleRateLimit.CompareAndSwap(true, false) {
		return fmt.Errorf("trading rate-limiter was already stopped or never initialized")
	}
	return nil
}

// StartBalanceManager gets current account balances from Kraken's REST API
// and loads them into memory, then it starts a go routine which listens for
// completed trades from the "ownTrades" WebSocket channel and updates the
// balances accordingly. Must be used with an open subscription to "ownTrades"
// channel. This feature only tracks total balances. It does not track available
// balances for trading.
//
// Note: Recommended use is to call SubscribeOwnTrades passing WithoutSnapshot
// to 'options' before StartBalanceManager as the snapshot message from the
// "ownTrades" channel will cause balances to be inaccurate. If snapshot message
// is required for some other reason, then call StartBalanceManager after
// SubscribeOwnTrades and before any trading occurs.
func (kc *KrakenClient) StartBalanceManager() error {
	bm := kc.WebSocketManager.BalanceMgr
	bm.mutex.Lock()
	defer bm.mutex.Unlock()

	// Check if BalanceManager is already running
	if bm.isActive {
		return fmt.Errorf("BalanceMgr already started")
	}

	// Get and convert initial balances to decimals, insert them into Balances field
	startingBalances, err := kc.GetAccountBalances()
	if err != nil {
		return fmt.Errorf("error getting initial balances from REST API")
	}
	if bm.Balances == nil {
		bm.Balances = make(map[string]decimal.Decimal, len(*startingBalances))
	}
	if bm.CurrencyMap == nil {
		bm.CurrencyMap = make(map[string]BaseQuoteCurrency)
	}
	for currency, balance := range *startingBalances {
		decBalance, err := decimal.NewFromString(balance)
		if err != nil {
			return fmt.Errorf("error converting initial balance to decimal")
		}
		bm.Balances[currency] = decBalance
	}

	// Initialize channel and start receiver go routine to process and update balances
	bm.ch = make(chan WSOwnTrade)
	ctx, cancel := context.WithCancel(context.Background())
	bm.ctx = ctx
	bm.cancel = cancel
	go func() {
		for {
			select {
			case <-ctx.Done():
				bm.mutex.Lock()
				bm.isActive = false
				close(bm.ch)
				bm.mutex.Unlock()
				return
			case data := <-bm.ch:
				bm.mutex.Lock()

				// Add pair to currency map if not exists
				_, ok := bm.CurrencyMap[data.Pair]
				if !ok {
					pairInfo, err := kc.GetTradeablePairsInfo(data.Pair)
					if err != nil {
						kc.WebSocketManager.ErrorLogger.Printf("error getting tradeable pairs info for %s, internal balances may be incorrect", data.Pair)
					} else {
						for _, pair := range *pairInfo {
							newCurrency := BaseQuoteCurrency{
								Base:  pair.Base,
								Quote: pair.Quote,
							}
							bm.CurrencyMap[data.Pair] = newCurrency
							break
						}
					}
				}
				currency := bm.CurrencyMap[data.Pair]

				// Add and subtract vol, cost, and fees from balances
				switch data.Direction {
				case "buy":
					decVol, err := decimal.NewFromString(data.Volume)
					if err != nil {
						kc.WebSocketManager.ErrorLogger.Println("error converting volume from string to decimal, internal balances may be incorrect")
					} else {
						// add vol to currency base balance
						if _, ok := bm.Balances[currency.Base]; ok { // balance exists, add
							bm.Balances[currency.Base] = bm.Balances[currency.Base].Add(decVol)
						} else { // balance doesn't exist, initialize
							bm.Balances[currency.Base] = decVol
						}
					}
					decCost, err := decimal.NewFromString(data.QuoteCost)
					if err != nil {
						kc.WebSocketManager.ErrorLogger.Println("error converting cost from string to decimal, internal balances may be incorrect")
					} else {
						// subtract cost from currency quote balance
						if _, ok := bm.Balances[currency.Quote]; ok { // balance exists, substract
							bm.Balances[currency.Quote] = bm.Balances[currency.Quote].Sub(decCost)
						} else { // balance doesn't exist, initialize (it should exist or this will be negative)
							kc.WebSocketManager.ErrorLogger.Println("error finding balance for quote currency, initializing negative balance")
							bm.Balances[currency.Quote] = decCost.Neg()
						}
					}
					// subtract fee from currency quote balance
					decFee, err := decimal.NewFromString(data.QuoteFee)
					if err != nil {
						kc.WebSocketManager.ErrorLogger.Println("error converting fee from string to decimal, internal balances may be incorrect")
					} else {
						bm.Balances[currency.Quote] = bm.Balances[currency.Quote].Sub(decFee)
					}
				case "sell":
					decVol, err := decimal.NewFromString(data.Volume)
					if err != nil {
						kc.WebSocketManager.ErrorLogger.Println("error converting volume from string to decimal, internal balances may be incorrect")
					} else {
						// subtract vol from currency base balance
						if _, ok := bm.Balances[currency.Base]; ok { // balance exists, subtract
							bm.Balances[currency.Base] = bm.Balances[currency.Base].Sub(decVol)
						} else { // balance doesn't exist, initialize (it should exist or this will be negative)
							kc.WebSocketManager.ErrorLogger.Println("error finding balance for base currency, initializing negative balance")
							bm.Balances[currency.Base] = decVol.Neg()
						}
					}
					decCost, err := decimal.NewFromString(data.QuoteCost)
					if err != nil {
						kc.WebSocketManager.ErrorLogger.Println("error converting cost from string to decimal, internal balances may be incorrect")
					} else {
						// add cost to currency quote balance
						if _, ok := bm.Balances[currency.Quote]; ok { // balance exists, add
							bm.Balances[currency.Quote] = bm.Balances[currency.Quote].Add(decCost)
						} else { // balance doesn't exist, initialize
							bm.Balances[currency.Quote] = decCost
						}
					}
					// subtract fee from currency quote balance
					decFee, err := decimal.NewFromString(data.QuoteFee)
					if err != nil {
						kc.WebSocketManager.ErrorLogger.Println("error converting fee from string to decimal, internal balances may be incorrect")
					} else {
						bm.Balances[currency.Quote] = bm.Balances[currency.Quote].Sub(decFee)
					}
				}
				bm.mutex.Unlock()
			}
		}
	}()
	bm.isActive = true
	return nil
}

// StopBalanceManager stops balance manager feature by changing its flag to
// false and calling its context cancel function. Returns an error if balance
// manager is already stopped.
func (ws *WebSocketManager) StopBalanceManager() error {
	if !ws.BalanceMgr.isActive {
		return fmt.Errorf("error stopping balance manager, balance manager already stopped or never active")
	}
	ws.BalanceMgr.cancel()
	return nil
}

// #endregion

// #region Exported methods for WebSocket Connection (Connect<>, WaitForConnect)

// Creates both authenticated and public connections to Kraken WebSocket server.
// Use ConnectPublic() or ConnectPrivate() instead if only channel type is needed.
// Accepts arg 'systemStatusCallback' callback function which an end user can
// implement their own logic on handling incoming system status change messages.
//
// Note: Creates authenticated token with AuthenticateWebSockets() method which
// expires within 15 minutes. Strongly recommended to subscribe to at least one
// private WebSocket channel and leave it open.
//
// Note: systemStatusCallback will be triggered twice when Connect() is used as
// it connects to both an authenticated and a public server. Methods called here
// intended for initial startup may be triggered twice, recommended to include
// logic to prevent this or put subscription and startup methods in main after
// a call to WaitForConnect() instead.
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
//	online := make(chan struct{})
//	systemStatusCallback := func(status string) {
//		if status == "online" {
//			// starts the goroutine and subscribe to the book the first time an online message is received
//			_, ok := <-online
//			if ok {
//				close(online)
//				err = kc.SubscribeBook(pair, depth, nil)
//				go placeBidAskSpread(ctx)
//			}
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

// WaitForConnect blocks until the WebSocketManager has established all connections
// as confirmed by a "systemStatus" message (regardless of status) received from
// the WebSocket server, or until a timeout of 5 seconds has passed. It returns an
// error if the timeout is reached before all connections are established. If
// performing specific operations depending on system status message received is
// desired, write a systemStatusCallback func passed to the Connect() method instead.
// Intended to use to wait until connections are confirmed before subscribing to
// Kraken's WebSocket channels. Accepts none or one optional arg passed to
// 'timeoutMilliseconds' which is the number of milliseconds until this method
// times out and returns an error. Defaults to 5000 (5 seconds) if no args passed.
//
// # Example Usage:
//
//	kc, err := NewKrakenClient(apiKey, secretKey, 2)
//	err = kc.Connect(nil)
//	err = kc.WaitForConnect()
//	err = kc.SubscribeBook("XBT/USD", 10, nil)
//	err = kc.SubscribeOwnTrades(nil)
//	err = kc.WaitForSubscriptions()
//	//...start your program logic here...//
func (ws *WebSocketManager) WaitForConnect(timeoutMilliseconds ...uint16) error {
	timeout := time.Millisecond * 5000
	if len(timeoutMilliseconds) > 0 {
		if len(timeoutMilliseconds) > 1 {
			return fmt.Errorf("too many arguments passed, expected 0 or 1")
		}
		timeout = time.Millisecond * time.Duration(timeoutMilliseconds[0])
	}
	done := make(chan struct{})
	go func() {
		ws.ConnectWaitGroup.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout reached; check subscriptions and try again")
	}
}

// Disconnected returns true if any clients become disconnected after initial
// startup.
func (ws *WebSocketManager) Disconnected() bool {
	return ws.reconnectMgr.numDisconnected.Load() != 0
}

// WaitForReconnect blocks and waits for reconnected condition broadcasted after
// all clients are reconnected. Includes another call to Disconnected to prevent
// blocking in case reconnect happened during your shutdown/pause logic
func (ws *WebSocketManager) WaitForReconnect() {
	if ws.Disconnected() {
		ws.reconnectMgr.mutex.Lock()
		ws.reconnectMgr.reconnectCond.Wait()
		ws.reconnectMgr.mutex.Unlock()
	}
}

// #endregion

// #region Exported methods for Subscriptions (Subscribe<Channel>, Unsubscribe<Channel>, WaitForSubscriptions)

// Subscribes to "ticker" WebSocket channel for arg 'pair'. May pass a valid
// function to arg 'callback' to dictate what to do with incoming data to the
// channel or pass a nil callback if you choose to handle channels manually.
// Accepts up to one functional options arg 'options' for reqID.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one solution or the other.
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
// one solution or the other.
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
// one solution or the other.
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
// one solution or the other.
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

// SubscribeBook subscribes to Kraken's "book" WebSocket channel for arg 'pair'
// and specified 'depth'. Where 'depth' is number of levels shown for each bids
// and asks side. On subscribing, the first message received will be the full
// initial state of the order book. Call StartOrderBookManager() method before
// SubscribeBook to maintain current state of book in memory.
//
// May pass either a valid function to arg 'callback' to dictate what to do with
// incoming data to the channel or pass a nil callback if you choose to read/handle
// channel data manually. Accepts none or many functional options args 'options'.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one solution or the other.
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
	if ws.OrderBookMgr.isTracking.Load() {
		ws.SubscriptionMgr.SubscribeWaitGroup.Add(1)
		if callback == nil {
			callback = ws.bookCallback(channelName, pair, depth, nil)
			err = ws.subscribePublic(channelName, payload, pair, callback)
			if err != nil {
				return fmt.Errorf("error calling subscribepublic method | %w", err)
			}
		} else {
			callback2 := ws.bookCallback(channelName, pair, depth, callback)
			err = ws.subscribePublic(channelName, payload, pair, callback2)
			if err != nil {
				return fmt.Errorf("error calling subscribepublic method | %w", err)
			}
		}
	} else {
		err = ws.subscribePublic(channelName, payload, pair, callback)
		if err != nil {
			return fmt.Errorf("error calling subscribepublic method | %w", err)
		}
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

// Subscribes to "ownTrades" authenticated WebSocket channel. May pass either
// a valid function to arg 'callback' to dictate what to do with incoming data
// to the channel or pass a nil callback if you choose to read/handle channel
// data manually. Accepts none or many functional options args 'options'.
//
// CAUTION: Passing both a non-nil callback function and reading the channels
// manually in your code will result in conflicts reading incoming data. Choose
// one solution or the other.
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
		case SnapshotOption:
			option.Apply(&subscriptionBuffer)
			if ws.TradeLogger != nil {
				ws.TradeLogger.startSeq = 0
			}
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
// one solution or the other.
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
	if ws.OpenOrdersMgr != nil && ws.OpenOrdersMgr.isTracking.Load() {
		ws.OpenOrdersMgr.Mutex.Lock()
		ws.OpenOrdersMgr.seq = 0
		ws.OpenOrdersMgr.Mutex.Unlock()
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

// WaitForSubscriptions blocks until all subscriptions (both private and public)
// by the WebSocketManager have received a "subscribed" event type message
// from Kraken WebSocket server or until a timeout of 5 seconds has passed.
// It returns an error if the timeout is reached before all subscriptions are
// received. Accepts none or one optional arg passed to 'timeoutMilliseconds'
// which is the number of milliseconds until this method times out and returns
// an error. Defaults to 5000 (5 seconds) if no args passed.
//
// Note: SubscribeBook() calls perform an additional wait group increment if
// state of book is being managed internally. The wait group is decremented after
// initial state of book is built in memory. This prevents race errors if the
// program following logic requires state of book.
//
// # Example Usage:
//
//	kc, err := NewKrakenClient(apiKey, secretKey, 2)
//	err = kc.StartOrderBookManager()
//	err = kc.Connect(nil)
//	err = kc.WaitForConnect()
//	err = kc.SubscribeBook("XBT/USD", 10, nil)
//	err = kc.SubscribeOwnTrades(nil)
//	err = kc.WaitForSubscriptions()
//	//...start your program logic here...//
//	bookState, err = GetBookState("XBT/USD", 10) // safe to call "immediately"
func (ws *WebSocketManager) WaitForSubscriptions(timeoutMilliseconds ...uint16) error {
	timeout := time.Millisecond * 5000
	if len(timeoutMilliseconds) > 0 {
		if len(timeoutMilliseconds) > 1 {
			return fmt.Errorf("too many arguments passed, expected 0 or 1")
		}
		timeout = time.Millisecond * time.Duration(timeoutMilliseconds[0])
	}
	done := make(chan struct{})
	go func() {
		ws.SubscriptionMgr.SubscribeWaitGroup.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout reached; check subscriptions and try again")
	}
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

// #endregion

// #region Exported methods for WSOrders  (addOrder, editOrder, cancelOrder(s), TradingRateLimiter, LimitChase)

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
	if _, ok := validDirection[direction]; !ok {
		return fmt.Errorf("invalid arg '%s' passed to 'direction'; expected \"buy\" or \"sell\"", direction)
	}

	// Build payload
	var buffer bytes.Buffer
	orderType(&buffer)
	for _, option := range options {
		option(&buffer)
	}
	event := "addOrder"
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "type": "%s", "volume": "%s", "pair": "%s"%s}`, event, ws.WebSocketToken, direction, volume, pair, buffer.String())

	// Determine incrementAmount and rate limit if rate limiting is turned on
	if ws.TradingRateLimiter.HandleRateLimit.Load() {
		incrementAmount := int32(1)
		ws.tradingRateLimit(pair, incrementAmount)
	}

	// Write message to Kraken
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

// WSEditOrder Sends an edit order request for the order with 'orderID' and 'pair'.
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
	// Build payload
	var buffer bytes.Buffer
	for _, option := range options {
		option(&buffer)
	}
	event := "editOrder"
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "orderid": "%s", "pair": "%s"%s}`, event, ws.WebSocketToken, orderID, pair, buffer.String())

	// Determine incrementAmount and rate limit if rate limiting is turned on
	if ws.TradingRateLimiter.HandleRateLimit.Load() {
		var incrementAmount int32
		if ws.OpenOrdersMgr != nil && ws.OpenOrdersMgr.isTracking.Load() {
			order, ok := ws.OpenOrdersMgr.OpenOrders[orderID]
			if !ok {
				ordersWithUserRef := []WSOpenOrder{}
				for _, o := range ws.OpenOrdersMgr.OpenOrders {
					if fmt.Sprintf("%v", o.UserRef) == orderID {
						ordersWithUserRef = append(ordersWithUserRef, o)
					}
				}
				switch len(ordersWithUserRef) {
				case 0:
					return fmt.Errorf("open order with id/userRef %s not found", orderID)
				case 1:
					order = ordersWithUserRef[0]
				default:
					return fmt.Errorf("userref passed to 'orderID' linked with multiple orders, try calling WSEditOrder again with unique orderID instead")
				}
			}
			openTime, err := strconv.ParseInt(order.OpenTime, 10, 64)
			if err != nil {
				return fmt.Errorf("error converting string to int64")
			}
			timeNow := time.Now().Unix()
			timeSince := timeNow - openTime
			switch {
			case timeSince < 5:
				incrementAmount = 6
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 10:
				incrementAmount = 5
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 15:
				incrementAmount = 4
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 45:
				incrementAmount = 2
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 90:
				incrementAmount = 1
				ws.tradingRateLimit(pair, incrementAmount)
			default:
				// 0 increment amount, do nothing
			}
		} else { // OpenOrdersMgr not running, assume the max increment amount
			incrementAmount = 6
			ws.tradingRateLimit(pair, incrementAmount)
		}
	}

	// Write message to Kraken
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
	// Build payload
	event := "cancelOrder"
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "txid": ["%s"]}`, event, ws.WebSocketToken, orderID)

	// Determine incrementAmount and rate limit if rate limiting is turned on
	if ws.TradingRateLimiter.HandleRateLimit.Load() {
		var incrementAmount int32
		if ws.OpenOrdersMgr != nil && ws.OpenOrdersMgr.isTracking.Load() {
			order, ok := ws.OpenOrdersMgr.OpenOrders[orderID]
			if !ok {
				return fmt.Errorf("open order with id %s not found", orderID)
			}
			pair := order.Description.Pair
			openTime, err := strconv.ParseInt(order.OpenTime, 10, 64)
			if err != nil {
				return fmt.Errorf("error converting string to int64")
			}
			timeNow := time.Now().Unix()
			timeSince := timeNow - openTime
			switch {
			case timeSince < 5:
				incrementAmount = 8
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 10:
				incrementAmount = 6
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 15:
				incrementAmount = 5
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 45:
				incrementAmount = 4
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 90:
				incrementAmount = 2
				ws.tradingRateLimit(pair, incrementAmount)
			case timeSince < 300:
				incrementAmount = 1
				ws.tradingRateLimit(pair, incrementAmount)
			default:
				// 0 increment amount, do nothing
			}
		} // else OpenOrdersMgr not running, cannot determine pair, cannot rate limit
	}

	// Write message to Kraken
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
	// Build payload
	event := "cancelOrder"
	ordersJSON, err := json.Marshal(orderIDs)
	if err != nil {
		return fmt.Errorf("error marshalling order ids to json | %w", err)
	}
	payload := fmt.Sprintf(`{"event": "%s", "token": "%s", "txid": %s}`, event, ws.WebSocketToken, string(ordersJSON))

	// Determine incrementAmount and rate limit if rate limiting is turned on
	if ws.TradingRateLimiter.HandleRateLimit.Load() {
		if ws.OpenOrdersMgr != nil && ws.OpenOrdersMgr.isTracking.Load() {
			pairsIncrementAmounts := make(map[string]int32)
			for _, orderID := range orderIDs {
				order, ok := ws.OpenOrdersMgr.OpenOrders[orderID]
				if !ok {
					return fmt.Errorf("open order with id %s not found", orderID)
				}
				pair := order.Description.Pair
				openTime, err := strconv.ParseInt(order.OpenTime, 10, 64)
				if err != nil {
					return fmt.Errorf("error converting string to int64")
				}
				timeNow := time.Now().Unix()
				timeSince := timeNow - openTime
				switch {
				case timeSince < 5:
					pairsIncrementAmounts[pair] += 8
				case timeSince < 10:
					pairsIncrementAmounts[pair] += 6
				case timeSince < 15:
					pairsIncrementAmounts[pair] += 5
				case timeSince < 45:
					pairsIncrementAmounts[pair] += 4
				case timeSince < 90:
					pairsIncrementAmounts[pair] += 2
				case timeSince < 300:
					pairsIncrementAmounts[pair] += 1
				default:
					// 0 increment amount, do nothing
				}
			}
			for pair, incrementAmount := range pairsIncrementAmounts {
				ws.tradingRateLimit(pair, incrementAmount)
			}
		} // else OpenOrdersMgr not running, cannot determine pair, cannot rate limit
	}

	// Write message to Kraken
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
	// Build payload
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
	// Build payload
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

// WSLimitChase gets top of book prices and places a post-only limit order on the
// exchange, and then it manages the order by editing or cancel and replacing
// as necessary (if the order is partially filled) when the top of book level
// changes.
//
// This method first retrieves top of book price for the given arg passed to
// 'pair' and places an order in the given 'direction' ("buy" or "sell") for
// the specified quantity/size passed to 'volume'. This method requires a
// unique user reference ID passed to 'userRef' that is not used for any other
// open orders whether made by this package or otherwise. This method requires
// internal management of orderbooks (use StartOrderBookManager()) a
// pre-existing active WebSocket subscription to the private "openOrders"
// channel, and a subscription to a public "book" channel for the given 'pair'.
//
// Depth for the "book" channel can be any valid depth, but it is recommended
// to have only one depth subscribed per 'pair'. Larger depths will incur more
// unnecessary processing.
//
// The 'fillCallBack' arg is a custom function where the end user can dictate
// what to do when a fill (partial or full) happens. This arg can be nil. The
// 'closeCallback' is the same, but gets called whenever the limit chase order
// ends whether it be closed due to order fully filled, errors encountered, or
// otherwise.
//
// # Enums:
//
// 'direction' - "buy", "sell"
//
// 'volume' - positive number value expressed as a string in quote currency. Must be
// above minimum size for 'pair' and have correct decimals. Use GetTradeablePairsInfo(pair)
// for 'pair' specific details
//
// 'pair' - valid tradeable pair and format for websocket. Use ListWebsocketNames()
// to get a slice of possible tradeable pairs
//
// 'userRef' - unique identifier not used for any other order. Will not return
// immediately if 'userRef' is not unique, will place initial order and then
// cancel both the limit-chase order and all other orders with the same 'userRef'
// and return with error.
//
// # Example Usage:
//
// Error handling omitted throughout
//
//	// Typical client initialization and subscribe required channels
//	direction := "buy"
//	oppDirection := "sell"
//	krakenPair := "XBT/USD"
//	liquidExchangePair := "BTCUSDT"
//	depth := uint16(10)
//	volume := "0.0001"
//	userRef := 10010032
//	kc, err := krakenspot.NewKrakenClient(apiKey, apiSecret, 2)
//	err = kc.StartOrderBookManager()
//	subscriptionsDone := make(chan bool)
//	systemStatusCallback := func(status string) {
//		if status == "online" {
//			err = kc.SubscribeBook(krakenPair, depth, nil)
//			err = kc.SubscribeOpenOrders(nil)
//			close(subscriptionsDone)
//		} else { // system not online
//			close(subscriptionsDone)
//			return
//		}
//	}
//	err = kc.Connect(systemStatusCallback)
//	<-subscriptionsDone
//	limitChaseClosed := make(chan bool)
//	fillCallback := func(lcFill *krakenspot.LimitChaseFill) {
//		bc.MarketOrderOnLiquidExchange(oppDirection, lcFill.FilledVol.String(), liquidExchangePair)
//	}
//	closeCallback := func() {
//		log.Println("limit-chase order closed")
//		close(limitChaseClosed)
//	}
//	kc.WSLimitChase(direction, volume, krakenPair, userRef, fillCallback, closeCallback)
//	<-limitChaseClosed
//	return
func (ws *WebSocketManager) WSLimitChase(direction, volume, pair string, userRef int32, fillCallback func(*LimitChaseFill), closeCallback func()) error {
	// Validate inputs and convert 'volume' to decimal 'direction' to int
	dir, ok := validDirection[direction]
	if !ok {
		return fmt.Errorf("invalid arg '%s' passed to 'direction'; expected \"buy\" or \"sell\"", direction)
	}
	ws.LimitChaseMgr.Mutex.RLock()
	if _, ok := ws.LimitChaseMgr.LimitChaseOrders[userRef]; ok {
		ws.LimitChaseMgr.Mutex.RUnlock()
		return fmt.Errorf("unique userRef required; limit chase order with userRef %v already exists", userRef)
	}
	ws.LimitChaseMgr.Mutex.RUnlock()
	decVol, err := decimal.NewFromString(volume)
	if err != nil {
		return fmt.Errorf("error converting volume to decimal")
	}

	// Check required subscriptions exist
	if !ws.OrderBookMgr.isTracking.Load() {
		return fmt.Errorf("must use StartOrderBookManager() before subscribing to book")
	}
	ws.SubscriptionMgr.Mutex.RLock()
	depth := uint16(10)
	if _, ok := ws.SubscriptionMgr.PublicSubscriptions["book-10"][pair]; !ok {
		bookSubscribed := false
		for book := range ws.SubscriptionMgr.PublicSubscriptions {
			if _, ok := ws.SubscriptionMgr.PublicSubscriptions[book][pair]; ok {
				_, depthCut, _ := strings.Cut(book, "-")
				depth64, err := strconv.ParseUint(depthCut, 10, 16)
				if err != nil {
					ws.SubscriptionMgr.Mutex.RUnlock()
					return fmt.Errorf("error parsing uint")
				}
				depth = uint16(depth64)
				bookSubscribed = true
				break
			}
		}
		if !bookSubscribed {
			ws.SubscriptionMgr.Mutex.RUnlock()
			return fmt.Errorf("no active book subscription for pair \"%s\"", pair)
		}
	}
	if _, ok := ws.SubscriptionMgr.PrivateSubscriptions["openOrders"]; !ok {
		ws.SubscriptionMgr.Mutex.RUnlock()
		return fmt.Errorf("no active \"openOrders\" subscription")
	}
	ws.SubscriptionMgr.Mutex.RUnlock()

	// Initialize new LimitChase and add to LimitChaseOrders map
	ctx, cancel := context.WithCancel(context.Background())
	newLC := &LimitChase{
		userRef:         userRef,
		userRefStr:      fmt.Sprintf("%v", userRef),
		pair:            pair,
		direction:       dir,
		startingVolume:  decVol,
		remainingVol:    decVol,
		filledVol:       decimal.Zero,
		orderPrice:      decimal.Zero,
		pending:         true,
		partiallyFilled: false,
		fullyFilled:     false,
		dataChan:        make(chan interface{}, 10),
		dataChanOpen:    true,
		fillCallback:    fillCallback,
		closeCallback:   closeCallback,
		ctx:             ctx,
		cancel:          cancel,
	}
	ws.LimitChaseMgr.Mutex.Lock()
	ws.LimitChaseMgr.LimitChaseOrders[userRef] = newLC
	ws.LimitChaseMgr.Mutex.Unlock()

	// start workers to process messages and update orders
	workerCount := 5
	for i := 0; i < workerCount; i++ {
		go ws.processLimitChaseWorker(ctx, newLC, direction, depth)
	}

	// handle cancellation
	go func() {
		<-ctx.Done()
		err = ws.WSCancelOrder(newLC.userRefStr)
		if err != nil {
			ws.ErrorLogger.Printf("error encountered closing limit chase | error cancelling order with userref %s; must cancel manually\n", newLC.userRefStr)
		}
		ws.closeAndDeleteLimitChase(userRef)
	}()

	// get top of book price and send initial order
	bookState, err := ws.GetBookState(pair, depth)
	if err != nil {
		ws.closeAndDeleteLimitChase(newLC.userRef)
		return fmt.Errorf("error encountered during limit chase | error getting book state; closing | %w", err)
	}
	var topOfBook decimal.Decimal
	switch dir {
	case 1: // "buy"
		topOfBook = (*bookState.Bids)[0].Price
	case -1: // "sell"
		topOfBook = (*bookState.Asks)[0].Price
	}
	err = ws.WSAddOrder(WSLimit(topOfBook.String()), direction, newLC.startingVolume.String(), pair, WSPostOnly(), WSAddOrderReqID(newLC.userRefStr), WSUserRef(newLC.userRefStr))
	if err != nil {
		ws.closeAndDeleteLimitChase(newLC.userRef)
		return fmt.Errorf("error encountered during limit chase | error sending first order | %w", err)
	}
	return nil
}

// CancelLimitChase cancels a LimitChase order with the same user reference ID
// passed to 'userRef' and removes it from the LimitChaseOrders map. The method
// ensures thread safety by using a mutex lock when accessing the map. It returns
// an error if a LimitChase order with the given 'userRef' doesn't exist.
func (ws *WebSocketManager) CancelLimitChase(userRef int32) error {
	if lc, ok := ws.LimitChaseMgr.LimitChaseOrders[userRef]; !ok {
		return fmt.Errorf("limit chase order with userRef %v doesn't exist", userRef)
	} else {
		lc.cancel()
	}
	return nil
}

// #endregion

// #region Exported methods for Features interacting with *WebSocketManager   ()

// GetBookState returns a BookState struct which holds pointers to both Bids
// and Asks slices for current state of book for arg 'pair' and specified 'depth'.
//
// Note: This method should only be called when current state of book is being
// managed by the WebSocketManager within the KrakenClient instance (when
// StartOrderBookManager() method was called before subscription). This method
// will throw an error if no subscriptions were made after StartOrderBookManager()
// was called.
//
// CAUTION: As this method returns pointers to the Asks and Bids fields, any
// modification done directly to Asks or Bids will likely result in an error and
// cause the WebSocketManager to unsubscribe from this channel. If modification
// to these slices is desired, either make a copy from reference or use ListAsks()
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

// ListAsks returns a copy of the Asks slice which holds the current state of the order
// book for arg 'pair' and specified 'depth'. May be less performant than GetBookState()
// for larger lists ('depth').
//
// Note: This method should only be called when current state of book is being
// managed by the WebSocketManager within the KrakenClient instance (when
// StartOrderBookManager() method was called before subscription). This method
// will throw an error if no subscriptions were made after StartOrderBookManager()
// was called.
func (ws *WebSocketManager) ListAsks(pair string, depth uint16) ([]InternalBookEntry, error) {
	if _, ok := ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair]; !ok {
		return []InternalBookEntry{}, fmt.Errorf("error encountered | book for pair %s and depth %v does not exist; check inputs and/or subscriptions and try again", pair, depth)
	}
	return append([]InternalBookEntry(nil), ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair].Asks...), nil
}

// ListBids returns a copy of the Bids slice which holds the current state of the order
// book for arg 'pair' and specified 'depth'. May be less performant than GetBookState()
// for larger lists ('depth').
//
// Note: This method should only be called when current state of book is being
// managed by the WebSocketManager within the KrakenClient instance (when
// StartOrderBookManager() method was called before subscription). This method
// will throw an error if no subscriptions were made after StartOrderBookManager()
// was called.
func (ws *WebSocketManager) ListBids(pair string, depth uint16) ([]InternalBookEntry, error) {
	if _, ok := ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair]; !ok {
		return []InternalBookEntry{}, fmt.Errorf("error encountered | book for pair %s and depth %v does not exist; check inputs and/or subscriptions and try again", pair, depth)
	}
	return append([]InternalBookEntry(nil), ws.OrderBookMgr.OrderBooks[fmt.Sprintf("book-%v", depth)][pair].Bids...), nil
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
//	signal.Notify(sigs, os.Interrupt)
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

// AssetBalance retrieves total decimal balance for arg 'asset' when balance manager
// is active. Must be called after StartBalanceManager. Returns an error if
// either balance manager is not active or if 'asset' is not found.
//
// Note: Many assets have different tickers in Kraken's system than those listed
// in their currency pairs, use GetTradablePairsInfo to verify correct asset
// ticker's to pass to 'asset'.
func (ws *WebSocketManager) AssetBalance(asset string) (decimal.Decimal, error) {
	ws.BalanceMgr.mutex.RLock()
	defer ws.BalanceMgr.mutex.RUnlock()
	if ws.BalanceMgr.Balances != nil && ws.BalanceMgr.isActive {
		if bal, ok := ws.BalanceMgr.Balances[asset]; !ok {
			return decimal.Zero, fmt.Errorf("balance for asset %s not found", asset)
		} else {
			return bal, nil
		}
	}
	return decimal.Zero, fmt.Errorf("error finding balance manager, use StartBalanceManager before calling this method")
}

// AssetBalances retrieves a map for all total asset balances managed internally
// when balance manager is active. Must be called after StartBalanceManager.
// Returns an error if balance manager is not active.
func (ws *WebSocketManager) AssetBalances() (map[string]decimal.Decimal, error) {
	ws.BalanceMgr.mutex.RLock()
	defer ws.BalanceMgr.mutex.RUnlock()
	if ws.BalanceMgr.Balances != nil && ws.BalanceMgr.isActive {
		return ws.BalanceMgr.Balances, nil
	}
	return nil, fmt.Errorf("error finding balance manager, use StartBalanceManager before calling this method")
}

// #endregion

// #region helper methods for *WebSocketManager Functionality (subscribe, readers, routers, trading rate-limiter, limit-chase)

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
	// HARDCODED WORKAROUND to avoid panic from racing when back to back Unsubscribe()
	// and Subscribe() methods are called for the same channel
	// if channel/pair already exists, unlock mutex and sleep lets Unsubscribe()
	// processes finish deleting pair from book before creating a new one
	// FIXME code will likely be reached and slow down when resubscribing
	if _, ok := ws.SubscriptionMgr.PublicSubscriptions[channelName][pair]; ok {
		ws.SubscriptionMgr.Mutex.Unlock()
		time.Sleep(time.Millisecond * 300)
		ws.SubscriptionMgr.Mutex.Lock()
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
		ws.SubscriptionMgr.SubscribeWaitGroup.Add(1)
	}
	ws.WebSocketClient.Mutex.Unlock()

	// Start go routine listen for incoming data and call callback functions
	go func() {
		<-sub.ConfirmedChan // wait for subscription confirmed
		for {
			select {
			case <-ws.WebSocketClient.Ctx.Done():
				if sub.DoneChanClosed == 0 { // channel is open
					sub.closeChannels()
					// Delete subscription from map if not attempting reconnect
					if !ws.WebSocketClient.attemptReconnect {
						ws.SubscriptionMgr.Mutex.Lock()
						delete(ws.SubscriptionMgr.PublicSubscriptions[channelName], pair)
						ws.SubscriptionMgr.Mutex.Unlock()
					}
					return
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
			case data := <-sub.DataChan:
				if sub.DataChanClosed == 0 { // channel is open
					if sub.Callback != nil {
						sub.Callback(data)
					}
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
		ws.SubscriptionMgr.SubscribeWaitGroup.Add(1)
	}
	ws.AuthWebSocketClient.Mutex.Unlock()

	// Start go routine listen for incoming data and call callback functions
	switch channelName {
	case "ownTrades":
		go func() {
			<-sub.ConfirmedChan // wait for subscription confirmed
			for {
				select {
				case <-ws.AuthWebSocketClient.Ctx.Done():
					if sub.DoneChanClosed == 0 { // channel is open
						sub.closeChannels()
						// Delete subscription from map if not attempting reconnect
						if !ws.WebSocketClient.attemptReconnect {
							ws.SubscriptionMgr.Mutex.Lock()
							delete(ws.SubscriptionMgr.PrivateSubscriptions, channelName)
							ws.SubscriptionMgr.Mutex.Unlock()
						}
						return
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
				case data := <-sub.DataChan:
					if sub.DataChanClosed == 0 { // channel is open
						// send to trade logger if it is running
						if ws.TradeLogger != nil && ws.TradeLogger.isLogging.Load() {
							ownTrades, ok := data.(WSOwnTradesResp)
							if !ok {
								ws.ErrorLogger.Println("error asserting data to WSOwnTradesResp")
							} else {
								if ws.TradeLogger.seq+1 != ownTrades.Sequence {
									ws.TradeLogger.seqErrorCh <- fmt.Errorf("sequence out of order: expected %d but got %d", ws.TradeLogger.seq+1, ownTrades.Sequence)
								}
								ws.TradeLogger.seq = ownTrades.Sequence
								if ownTrades.Sequence > ws.TradeLogger.startSeq {
									for _, trade := range ownTrades.OwnTrades {
										ws.TradeLogger.ch <- trade
									}
								}
							}
						}
						// send to balance manager if it is running
						ws.BalanceMgr.mutex.RLock()
						if ws.BalanceMgr.isActive {
							ownTrades, ok := data.(WSOwnTradesResp)
							if !ok {
								ws.ErrorLogger.Println("error asserting data to WSOwnTradesResp")
							} else {
								for _, o := range ownTrades.OwnTrades {
									for _, t := range o {
										ws.BalanceMgr.ch <- t
									}
								}
							}
						}
						ws.BalanceMgr.mutex.RUnlock()
						// process user defined callback
						if sub.Callback != nil {
							sub.Callback(data)
						}
					}
				}
			}
		}()
	case "openOrders":
		go func() {
			<-sub.ConfirmedChan // wait for subscription confirmed
			for {
				select {
				case <-ws.AuthWebSocketClient.Ctx.Done():
					if sub.DoneChanClosed == 0 { // channel is open
						sub.closeChannels()
						// Delete subscription from map if not attempting reconnect
						if !ws.WebSocketClient.attemptReconnect {
							ws.SubscriptionMgr.Mutex.Lock()
							delete(ws.SubscriptionMgr.PrivateSubscriptions, channelName)
							ws.SubscriptionMgr.Mutex.Unlock()
						}
						return
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
				case data := <-sub.DataChan:
					if sub.DataChanClosed == 0 { // channel is open
						openOrders, ok := data.(WSOpenOrdersResp)
						if !ok {
							ws.ErrorLogger.Println("error asserting data to WSOpenOrdersResp")
						}
						if ws.OpenOrdersMgr != nil && ws.OpenOrdersMgr.isTracking.Load() {
							ws.OpenOrdersMgr.ch <- openOrders
						}
						if ws.TradingRateLimiter.HandleRateLimit.Load() {
							for _, orders := range openOrders.OpenOrders {
								for _, order := range orders {
									if _, ok := ws.TradingRateLimiter.Counters[order.Description.Pair]; ok {
										if ws.TradingRateLimiter.Counters[order.Description.Pair].Load() == 0 {
											ws.TradingRateLimiter.Counters[order.Description.Pair].Store(int32(order.RateCount))
											go ws.startTradeRateLimiter(order.Description.Pair)
										} else {
											ws.TradingRateLimiter.Counters[order.Description.Pair].Store(int32(order.RateCount))
										}
									} else { // first time pair has had an order sent
										ws.TradingRateLimiter.Counters[order.Description.Pair] = new(atomic.Int32)
										ws.TradingRateLimiter.CounterDecayConds[order.Description.Pair] = sync.NewCond(&sync.Mutex{})
									}
								}
							}
						}
						if sub.Callback != nil {
							sub.Callback(data)
						}
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
							if c.attemptReconnect {
								if c.isReconnecting.CompareAndSwap(false, true) {
									c.reconnector.numDisconnected.Add(1)
									c.reconnect(url)
								}
							}
							return
						} else if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseProtocolError, websocket.CloseUnsupportedData, websocket.CloseNoStatusReceived) {
							c.ErrorLogger.Println("unexpected websocket closure, attempting reconnect to url |", url)
							c.Cancel()
							c.Conn.Close()
							if c.attemptReconnect {
								if c.isReconnecting.CompareAndSwap(false, true) {
									c.reconnector.numDisconnected.Add(1)
									c.reconnect(url)
								}
							}
							return
						} else if strings.Contains(err.Error(), "wsarecv") {
							c.ErrorLogger.Println("internet connection lost, attempting reconnect to url |", url)
							c.Cancel()
							c.Conn.Close()
							if c.attemptReconnect {
								if c.isReconnecting.CompareAndSwap(false, true) {
									c.reconnector.numDisconnected.Add(1)
									c.reconnect(url)
								}
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
		// limit chase orders, send to appropriate channels if limit chase exists
		ws.LimitChaseMgr.Mutex.RLock()
		if len(ws.LimitChaseMgr.LimitChaseOrders) > 0 {
			for _, lc := range ws.LimitChaseMgr.LimitChaseOrders {
				if lc.pair == v.Pair {
					lc.mutex.RLock()
					if lc.dataChanOpen {
						select {
						case lc.dataChan <- v:
						default: // cancel limit chase if dataChan not able to receive
							ws.ErrorLogger.Println("*LimitChase.dataChan not able to receive message; cancelling limit-chase")
							ws.CancelLimitChase(lc.userRef)
						}
					}
					lc.mutex.RUnlock()
				}
			}
		}
		ws.LimitChaseMgr.Mutex.RUnlock()
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
		// Limit-chase orders; send to appropriate channel if limit chase exists
		ws.LimitChaseMgr.Mutex.RLock()
		if len(ws.LimitChaseMgr.LimitChaseOrders) > 0 {
			for _, orderMap := range v.OpenOrders {
				for _, order := range orderMap {
					if lc, ok := ws.LimitChaseMgr.LimitChaseOrders[int32(order.UserRef)]; ok {
						lc.mutex.RLock()
						if lc.dataChanOpen {
							select {
							case lc.dataChan <- v:
							default: // cancel limit chase if dataChan not able to receive
								ws.ErrorLogger.Println("*LimitChase.dataChan not able to receive message; cancelling limit-chase")
								ws.CancelLimitChase(lc.userRef)
							}
						}
						lc.mutex.RUnlock()
					}
				}
			}
		}
		ws.LimitChaseMgr.Mutex.RUnlock()
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
		// send to appropriate limit chase if exists
		ws.LimitChaseMgr.Mutex.RLock()
		if len(ws.LimitChaseMgr.LimitChaseOrders) > 0 {
			if lc, ok := ws.LimitChaseMgr.LimitChaseOrders[int32(v.RequestID)]; ok {
				lc.mutex.RLock()
				if lc.dataChanOpen {
					select {
					case lc.dataChan <- v:
					default: // cancel limit chase if dataChan not able to receive
						ws.ErrorLogger.Println("*LimitChase.dataChan not able to receive message; cancelling limit-chase")
						ws.CancelLimitChase(lc.userRef)
					}
				}
				lc.mutex.RUnlock()
			}
		}
		ws.LimitChaseMgr.Mutex.RUnlock()
	case WSEditOrderResp:
		if ws.OrderStatusCallback != nil {
			ws.OrderStatusCallback(v)
		}
		// send to appropriate limit chase if exists
		ws.LimitChaseMgr.Mutex.RLock()
		if len(ws.LimitChaseMgr.LimitChaseOrders) > 0 {
			if lc, ok := ws.LimitChaseMgr.LimitChaseOrders[int32(v.RequestID)]; ok {
				lc.mutex.RLock()
				if lc.dataChanOpen {
					select {
					case lc.dataChan <- v:
					default: // cancel limit chase if dataChan not able to receive
						ws.ErrorLogger.Println("*LimitChase.dataChan not able to receive message; cancelling limit-chase")
						ws.CancelLimitChase(lc.userRef)
					}
				}
				lc.mutex.RUnlock()
			}
		}
		ws.LimitChaseMgr.Mutex.RUnlock()
	case WSCancelOrderResp:
		if ws.OrderStatusCallback != nil {
			ws.OrderStatusCallback(v)
		}
		// send to appropriate limit chase if exists
		ws.LimitChaseMgr.Mutex.RLock()
		if len(ws.LimitChaseMgr.LimitChaseOrders) > 0 {
			if lc, ok := ws.LimitChaseMgr.LimitChaseOrders[int32(v.RequestID)]; ok {
				lc.mutex.RLock()
				if lc.dataChanOpen {
					select {
					case lc.dataChan <- v:
					default: // cancel limit chase if dataChan not able to receive
						ws.ErrorLogger.Println("*LimitChase.dataChan not able to receive message; cancelling limit-chase")
						ws.CancelLimitChase(lc.userRef)
					}
				}
				lc.mutex.RUnlock()
			}
		}
		ws.LimitChaseMgr.Mutex.RUnlock()
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
			ws.SubscriptionMgr.SubscribeWaitGroup.Done()
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
					if ws.OrderBookMgr.isTracking.Load() {
						ws.OrderBookMgr.OrderBooks[v.ChannelName][v.Pair].unsubscribe()
					}
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
		ws.ConnectWaitGroup.Done()
		if ws.SystemStatusCallback != nil {
			ws.SystemStatusCallback(v.Status)
		}
	case WSPong:
		ws.ErrorLogger.Println("pong | reqid: ", v.ReqID)
	case WSErrorResp:
		ws.ErrorLogger.Printf("error message: %s | reqid: %d\n", v.ErrorMessage, v.ReqID)
	default:
		return fmt.Errorf("cannot route unknown msg type | %s", msg.Event)
	}
	return nil
}

// closeAndDeleteLimitChase is a helper method to safely close the data channel
// of a LimitChase order, remove it from the LimitChaseOrders map, and call the
// closeCallback if one was provided on LimitChase creation. It takes a pointer
// to a LimitChase struct and a userRef as arguments. The method ensures thread
// safety by using a mutex lock when deleting the LimitChase order from the map.
func (ws *WebSocketManager) closeAndDeleteLimitChase(userRef int32) {
	ws.LimitChaseMgr.Mutex.Lock()
	lc, ok := ws.LimitChaseMgr.LimitChaseOrders[userRef]
	if !ok {
		return
	}
	lc.mutex.Lock()
	lc.dataChanOpen = false
	close(lc.dataChan)
	lc.mutex.Unlock()
	delete(ws.LimitChaseMgr.LimitChaseOrders, userRef)
	ws.LimitChaseMgr.Mutex.Unlock()
	if lc.closeCallback != nil {
		lc.closeCallback()
	}
}

// processLimitChaseWorker is a helper method that processes incoming messages
// on a *LimitChase's dataChan and depending on the message type and content,
// either sends an update (edit or cancel&replace) order, closes the LimitChase,
// or calls LimitChase callbacks. The dataChan gets incoming messages from two
// startMessageReader() go routines (one each of authenticated and public
// WebSocketClient) and may cause limit-chase to eject in routeOrderMessage(),
// routePublicMessage(), or routePrivateMessage() if dataChan is not able to
// receive messages, so a sufficient number workers and dataChan buffer is required.
func (ws *WebSocketManager) processLimitChaseWorker(ctx context.Context, newLC *LimitChase, direction string, depth uint16) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-newLC.dataChan:
			if !ok {
				return
			}
			switch msg := data.(type) {
			case WSOpenOrdersResp:
				for _, openOrder := range msg.OpenOrders {
					for _, order := range openOrder {
						switch {
						case order.Status == "pending":
							newLC.mutex.Lock()
							newLC.pending = false
							orderPrice, err := decimal.NewFromString(order.Description.Price)
							if err != nil { // not sure we need to cancel&quit here, what situations does kraken send invalid price on a pending order?
								ws.ErrorLogger.Println("error encountered during limit chase | error converting string to decimal; cancelling and closing |", err)
							}
							newLC.partiallyFilled = false
							newLC.orderPrice = orderPrice
							newLC.mutex.Unlock()
						case order.Status == "canceled" && order.CancelReason != "Order replaced":
							// This case should also be true when closeAndDeleteLimitChase cancels a partially filled order
							bookState, err := ws.GetBookState(newLC.pair, depth)
							if err != nil {
								ws.ErrorLogger.Println("error encountered during limit chase | error getting book state on order cancelled; shutting down, restart limit chase if desired")
								ws.closeAndDeleteLimitChase(newLC.userRef)
								return
							}
							newLC.mutex.Lock()
							newLC.pending = true
							var newPrice decimal.Decimal
							switch newLC.direction {
							case 1: // "buy"
								newPrice = (*bookState.Bids)[0].Price
							case -1: // "sell"
								newPrice = (*bookState.Asks)[0].Price
							}
							ws.WSAddOrder(WSLimit(newPrice.String()), direction, newLC.remainingVol.String(), newLC.pair, WSPostOnly(), WSAddOrderReqID(newLC.userRefStr), WSUserRef(newLC.userRefStr))
							newLC.mutex.Unlock()
						case order.Status == "closed":
							newLC.mutex.Lock()
							newLC.fullyFilled = true
							newLC.mutex.Unlock()
							ws.closeAndDeleteLimitChase(newLC.userRef)
							return
						case order.VolumeExecuted != "":
							volExec, err := decimal.NewFromString(order.VolumeExecuted)
							if err != nil { // not sure we need to cancel&quit here, what situations does kraken send VolumeExecuted on an order?
								ws.ErrorLogger.Println("error converting volume executed to decimal | ", order.VolumeExecuted)
							}
							if volExec.Cmp(decimal.Zero) != 0 {
								newLC.mutex.Lock()
								filledVol := volExec.Sub(newLC.filledVol)
								newLC.filledVol = volExec
								newLC.remainingVol = newLC.startingVolume.Sub(newLC.filledVol)
								fill := &LimitChaseFill{
									FilledVol:    filledVol,
									RemainingVol: newLC.remainingVol,
								}
								if newLC.fillCallback != nil {
									newLC.fillCallback(fill)
								}
								if newLC.remainingVol.Cmp(decimal.Zero) == 0 {
									newLC.fullyFilled = true
									// order fully filled; call fillCallback and close limit-chase
									newLC.mutex.Unlock()
									ws.closeAndDeleteLimitChase(newLC.userRef)
									return
								} else { // order partially filled, call fillCallback
									newLC.partiallyFilled = true
									newLC.mutex.Unlock()
								}
							}
						}
					}
				}
			case WSBookUpdateResp:
				bookState, err := ws.GetBookState(newLC.pair, depth)
				if err != nil {
					ws.ErrorLogger.Println("error encountered during limit chase | error getting book state; cancelling order and closing")
					err = ws.WSCancelOrder(newLC.userRefStr)
					if err != nil {
						ws.ErrorLogger.Printf("error encountered closing limit chase | error cancelling order with userref %s; must cancel manually\n", newLC.userRefStr)
					}
					ws.closeAndDeleteLimitChase(newLC.userRef)
					return
				}
				var newPrice decimal.Decimal
				switch newLC.direction {
				case 1: // "buy"
					newPrice = (*bookState.Bids)[0].Price
				case -1: // "sell"
					newPrice = (*bookState.Asks)[0].Price
				}
				newLC.mutex.Lock()
				if newLC.orderPrice.Cmp(newPrice) != 0 && !newLC.fullyFilled {
					err = ws.updateLimitChase(newLC, newPrice, newLC.pair)
					if err != nil {
						ws.ErrorLogger.Println("error encountered during limit chase | error updating price; cancelling order and closing")
						err = ws.WSCancelOrder(newLC.userRefStr)
						if err != nil {
							ws.ErrorLogger.Printf("error encountered closing limit chase | error cancelling order with userref %s; must cancel manually\n", newLC.userRefStr)
						}
						newLC.mutex.Unlock()
						ws.closeAndDeleteLimitChase(newLC.userRef)
						return
					}
				}
				newLC.mutex.Unlock()
			case WSAddOrderResp:
				if msg.Status != "ok" {
					if strings.Contains(msg.ErrorMessage, "Invalid arguments:volume") {
						// remaining volume less than min order size; or invalid volume closing
						ws.closeAndDeleteLimitChase(newLC.userRef)
						return
					} else if strings.Contains(msg.ErrorMessage, "Currency pair not supported") {
						// invalid tradeable pair passed to 'pair'; closing
						ws.ErrorLogger.Println("error encountered during limit chase | invalid arg passed to 'pair':", newLC.pair)
						ws.closeAndDeleteLimitChase(newLC.userRef)
						return
					} else if strings.Contains(msg.ErrorMessage, "Invalid price") {
						// invalid price passed, this should never happen; closing
						ws.ErrorLogger.Println("error encountered during limit chase | unexpected invalid price passed in limit chase", msg)
						ws.closeAndDeleteLimitChase(newLC.userRef)
						return
					}
				}
			case WSEditOrderResp:
				if msg.Status != "ok" {
					if strings.Contains(msg.ErrorMessage, "Userref linked with multiple orders") {
						// non-unique userref, cancels all orders with userref including the limit-chase; close
						ws.ErrorLogger.Println("error encountered during limit chase | non-unique id passed to 'userRef'; cancelling all orders with userRef", newLC.userRefStr)
						err := ws.WSCancelOrder(newLC.userRefStr)
						if err != nil {
							ws.ErrorLogger.Printf("error encountered closing limit chase | error cancelling order with userref %s; must cancel manually\n", newLC.userRefStr)
						}
						ws.closeAndDeleteLimitChase(newLC.userRef)
						return
					} else if strings.Contains(msg.ErrorMessage, "Unknown order") {
						// Order no longer exists OR pending flag checks are not working correctly
						ws.ErrorLogger.Println("error encountered during limit chase | order was not found for editing, order may have been cancelled by another process; closing limit-chase")
						ws.closeAndDeleteLimitChase(newLC.userRef)
						return
					}
				}
			case WSCancelOrderResp:
				if strings.Contains(msg.ErrorMessage, "Unknown order") {
					// Order no longer exists OR pending flag checks are not working correctly
					ws.ErrorLogger.Println("error encountered during limit chase | order was not found for cancelling, order may have been cancelled by another process; closing limit-chase")
					ws.closeAndDeleteLimitChase(newLC.userRef)
					return
				}
			}
		}
	}
}

// updateLimitChase updates the state of a LimitChase order based on the new top
// of book price and sends either an WSEditOrder with the newPrice or a
// WSCancelOrder if the LimitChase has been partially filled. It takes a pointer
// to a LimitChase struct, a new price, and a pair as arguments. If the
// LimitChase order is not pending and not partially filled, it sends an edit
// order request to the server. If the LimitChase order is partially filled,
// it cancels the existing order and updates the starting volume. The method
// returns an error if there's an issue with sending the edit or cancel order
// request to the server.
func (ws *WebSocketManager) updateLimitChase(lc *LimitChase, newPrice decimal.Decimal, pair string) error {
	if !lc.pending {
		if !lc.partiallyFilled {
			lc.pending = true
			err := ws.WSEditOrder(lc.userRefStr, pair, WSNewPostOnly(), WSNewPrice(newPrice.String()), WSEditOrderReqID(lc.userRefStr), WSNewUserRef(lc.userRefStr))
			if err != nil {
				return fmt.Errorf("error calling WSEditOrder | %w", err)
			}
		} else {
			lc.pending = true
			lc.partiallyFilled = false
			lc.startingVolume = lc.remainingVol
			lc.filledVol = decimal.Zero
			err := ws.WSCancelOrder(lc.userRefStr)
			if err != nil {
				return fmt.Errorf("error calling WSCancelOrder | %w", err)
			}
		}
	}
	return nil
}

// #endregion

// #region helper methods for *WebSocketManager Features (TradeLogger, OpenOrderManager, TradingRateLimiter)

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

// startOpenOrderManager() is a helper function meant to be run as a go routine. Starts a loop that reads
// from ws.OpenOrdersMgr.ch and either builds the initial state of the currently
// open orders or it updates the state removing and adding orders as necessary
func (ws *WebSocketManager) startOpenOrderManager() {
	ws.OpenOrdersMgr.wg.Add(1)
	defer ws.OpenOrdersMgr.wg.Done()

	for data := range ws.OpenOrdersMgr.ch {
		if data.Sequence != ws.OpenOrdersMgr.seq+1 {
			ws.ErrorLogger.Println("improper open orders sequence; unsubscribing channel and resubscribing")
			err := ws.UnsubscribeOpenOrders()
			if err != nil {
				ws.ErrorLogger.Println("error unsubscribing \"openOrders\"; shutting down open order manager |", err)
				return
			}
			time.Sleep(time.Millisecond * 300)
			err = ws.SubscribeOpenOrders(ws.SubscriptionMgr.PrivateSubscriptions["openOrders"].Callback, WithRateCounter())
			if err != nil {
				ws.ErrorLogger.Println("error resubscribing \"openOrders\"; shutting down open order manager |", err)
				return
			}
		} else {
			ws.OpenOrdersMgr.seq = ws.OpenOrdersMgr.seq + 1
			if ws.OpenOrdersMgr.seq == 1 {
				// Build initial state of open orders
				ws.OpenOrdersMgr.OpenOrders = make(map[string]WSOpenOrder, len(data.OpenOrders))
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

// tradingRateLimit is a helper method that waits for the counter to decrement
// if adding 'incrementAmount' to the counter for specified 'pair' would go over
// the max counter amount for the user's verification tier.
func (ws *WebSocketManager) tradingRateLimit(pair string, incrementAmount int32) {
	if _, ok := ws.TradingRateLimiter.Counters[pair]; ok {
		for ws.TradingRateLimiter.Counters[pair].Load()+incrementAmount >= ws.TradingRateLimiter.MaxCounter {
			ws.ErrorLogger.Println("Counter will exceed rate limit. Waiting")
			ws.TradingRateLimiter.CounterDecayConds[pair].Wait()
		}
	}
}

// startTradeRateLimiter is a go routine method which starts a timer that decays
// kc.APICounter every second by the amount in kc.APICounterDecay. Stops at 0.
func (ws *WebSocketManager) startTradeRateLimiter(pair string) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(ws.TradingRateLimiter.CounterDecay))
	defer ticker.Stop()
	for range ticker.C {
		test := atomic.Int32{}
		test.Load()
		ws.TradingRateLimiter.Mutex.Lock()
		if ws.TradingRateLimiter.Counters[pair].Load() > 0 {
			ws.TradingRateLimiter.Counters[pair].Add(-1)
			if ws.TradingRateLimiter.Counters[pair].Load() == 0 {
				ticker.Stop()
				ws.TradingRateLimiter.Mutex.Unlock()
				return
			}
		}
		ws.TradingRateLimiter.Mutex.Unlock()
		ws.TradingRateLimiter.CounterDecayConds[pair].Broadcast()
	}
}

// #endregion

// #region helper methods for Connections  (connect, dialKraken, reconnect, reauthenticate)

// Helper method that initializes WebSocketClient for public endpoints by
// dialing Kraken's server and starting its message reader.
func (kc *KrakenClient) connectPublic() error {
	kc.WebSocketManager.WebSocketClient = &WebSocketClient{
		Router:           kc.WebSocketManager,
		resubscriber:     kc.WebSocketManager,
		attemptReconnect: kc.autoReconnect,
		isReconnecting:   atomic.Bool{},
		ErrorLogger:      kc.ErrorLogger,
		managerWaitGroup: kc.WebSocketManager.ConnectWaitGroup,
		reconnector:      kc.WebSocketManager.reconnectMgr,
	}
	kc.WebSocketManager.WebSocketClient.isReconnecting.Store(false)
	err := kc.WebSocketManager.WebSocketClient.dialKraken(wsPublicURL)
	if err != nil {
		return fmt.Errorf("error dialing kraken public url | %w", err)
	}
	kc.WebSocketManager.ConnectWaitGroup.Add(1)
	kc.WebSocketManager.WebSocketClient.startMessageReader(wsPublicURL)
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
	kc.WebSocketManager.AuthWebSocketClient = &WebSocketClient{
		Router:           kc.WebSocketManager,
		Authenticator:    kc,
		resubscriber:     kc.WebSocketManager,
		attemptReconnect: kc.autoReconnect,
		isReconnecting:   atomic.Bool{},
		ErrorLogger:      kc.ErrorLogger,
		managerWaitGroup: kc.WebSocketManager.ConnectWaitGroup,
		reconnector:      kc.WebSocketManager.reconnectMgr,
	}
	kc.WebSocketManager.AuthWebSocketClient.isReconnecting.Store(false)
	err = kc.WebSocketManager.AuthWebSocketClient.dialKraken(wsPrivateURL)
	if err != nil {
		return fmt.Errorf("error dialing kraken private url | %w", err)
	}
	kc.WebSocketManager.ConnectWaitGroup.Add(1)
	kc.WebSocketManager.AuthWebSocketClient.startMessageReader(wsPrivateURL)
	return nil
}

// Attempts reconnect with dialKraken() method. Retries 5 times instantly then
// scales back to attempt reconnect once every ~8 seconds.
func (c *WebSocketClient) reconnect(url string) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		t := 1.0
		count := 0
		for {
			err := c.dialKraken(url)
			if err != nil {
				c.ErrorLogger.Printf("error re-dialing, trying again | %s\n", err)
			} else {
				// Need to add to WebSocketManager wg because reader always
				// calls wg.Done() on WSSystemStatus type messages
				c.managerWaitGroup.Add(1)
				c.reconnector.numDisconnected.Add(-1)
				c.isReconnecting.Store(false)
				if c.reconnector.numDisconnected.Load() == 0 {
					c.reconnector.mutex.Lock()
					c.reconnector.reconnectCond.Broadcast()
					c.reconnector.mutex.Unlock()
					c.ErrorLogger.Println("reconnect successful")
				}
				// Unblocks reconnect for rest of code
				wg.Done()
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
	// Waits for this clients reconnect successful
	wg.Wait()
	// Reauthenticate WebSocket token if client is Private
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
	err := c.resubscriber.resubscribe(url)
	if err != nil {
		return fmt.Errorf("error calling resubscribe | %w", err)
	}
	return nil
}

func (ws *WebSocketManager) resubscribe(url string) error {
	switch url {
	case wsPrivateURL:
		for channelName, sub := range ws.SubscriptionMgr.PrivateSubscriptions {
			switch channelName {
			case "ownTrades":
				ws.SubscribeOwnTrades(sub.Callback, WithoutSnapshot())
			case "openOrders":
				ws.SubscribeOpenOrders(sub.Callback, WithRateCounter())
			}
		}
	case wsPublicURL:
		for channelName, pairMap := range ws.SubscriptionMgr.PublicSubscriptions {
			switch {
			case channelName == "ticker":
				for pair, sub := range pairMap {
					ws.SubscribeTicker(pair, sub.Callback)
				}
			case channelName == "trade":
				for pair, sub := range pairMap {
					ws.SubscribeTrade(pair, sub.Callback)
				}
			case channelName == "spread":
				for pair, sub := range pairMap {
					ws.SubscribeSpread(pair, sub.Callback)
				}
			case strings.HasPrefix(channelName, "ohlc"):
				_, intervalStr, _ := strings.Cut(channelName, "-")
				interval, err := strconv.ParseUint(intervalStr, 10, 16)
				if err != nil {
					return fmt.Errorf("error parsing uint from interval string")
				}
				for pair, sub := range pairMap {
					ws.SubscribeOHLC(pair, uint16(interval), sub.Callback)
				}
			case strings.HasPrefix(channelName, "book"):
				_, depthStr, _ := strings.Cut(channelName, "-")
				depth, err := strconv.ParseUint(depthStr, 10, 16)
				if err != nil {
					return fmt.Errorf("error parsing uint from interval string")
				}
				for pair, sub := range pairMap {
					ws.SubscribeBook(pair, uint16(depth), sub.Callback)
				}
			default:
				return fmt.Errorf("unknown channelName")
			}

		}
	default:
		return fmt.Errorf("unknown url")
	}
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

// #region helper methods for *Subscription  (confirm, close, unsubscribe, and constructor)

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

// #region helper methods for *InternalOrderBook  (bookCallback, unsubscribe, close, buildInitial, checksum, update)

func (ws *WebSocketManager) bookCallback(channelName, pair string, depth uint16, callback GenericCallback) func(data interface{}) {
	return func(data interface{}) {
		if msg, ok := data.(WSBookUpdateResp); ok { // data is book update message
			ws.OrderBookMgr.OrderBooks[channelName][pair].DataChan <- msg
			if callback != nil {
				callback(msg)
			}
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
					ws.SubscriptionMgr.SubscribeWaitGroup.Done()
					ws.ErrorLogger.Printf("error building initial state of book; sending unsubscribe msg; try subscribing again | %s\n", err)
					err := ws.UnsubscribeBook(pair, depth)
					if err != nil {
						ws.ErrorLogger.Printf("error sending unsubscribe; shut down kraken client and reinitialize | %s\n", err)
					}
					return
				}
				ws.SubscriptionMgr.SubscribeWaitGroup.Done()
				go func() {
					ob := ws.OrderBookMgr.OrderBooks[channelName][pair]
					for {
						select {
						case <-ws.WebSocketClient.Ctx.Done():
							ws.closeAndDeleteBook(ob, pair, channelName)
							return
						case <-ob.DoneChan:
							ws.closeAndDeleteBook(ob, pair, channelName)
							return
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
						}
					}
				}()
				if callback != nil {
					callback(msg)
				}
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
		price := strings.TrimLeft(strings.ReplaceAll(ob.Asks[i].Price.StringFixed(ob.PriceDecimals), ".", ""), "0")
		buffer.WriteString(price)
		volume := strings.TrimLeft(strings.ReplaceAll(ob.Asks[i].Volume.StringFixed(ob.VolumeDecimals), ".", ""), "0")
		buffer.WriteString(volume)
	}

	// write to buffer from bids
	for i := 0; i < 10; i++ {
		price := strings.TrimLeft(strings.ReplaceAll(ob.Bids[i].Price.StringFixed(ob.PriceDecimals), ".", ""), "0")
		buffer.WriteString(price)
		volume := strings.TrimLeft(strings.ReplaceAll(ob.Bids[i].Volume.StringFixed(ob.VolumeDecimals), ".", ""), "0")
		buffer.WriteString(volume)
	}
	ob.Mutex.RUnlock()
	if checksum := crc32.ChecksumIEEE(buffer.Bytes()); checksum != msgChecksum {
		return fmt.Errorf("invalid checksum | got: %v expected: %v\n orderbook: %v", checksum, msgChecksum, ob)
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
	_, priceDecStr, _ := strings.Cut(msg.Bids[0].Price, ".")
	_, volumeDecStr, _ := strings.Cut(msg.Bids[0].Volume, ".")
	ob.PriceDecimals = int32(len(priceDecStr))
	ob.VolumeDecimals = int32(len(volumeDecStr))
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

// #region helper functions

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
