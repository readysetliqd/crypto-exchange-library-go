// Package krakenspot is a comprehensive toolkit for interfacing with the Kraken
// Spot Exchange API. It enables WebSocket and REST API interactions, including
// subscription to both public and private channels. The package provides a
// client for initiating these interactions and a state manager for handling
// them.
//
// The krakenclient.go file specifically contains the declaration of data structs
// for the client, the client constructor function, and a method to set an error
// logger. It plays a crucial role in establishing and managing interactions
// with Kraken's WebSocket server, including subscribing to public and private
// channels and sending orders.
package krakenspot

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

var sharedClient = &http.Client{
	Timeout: time.Second * 10, // Set a 10-second timeout for requests
	// You can also set more granular timeouts using a custom http.Transport
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second, // Time spent establishing a TCP connection
			KeepAlive: 5 * time.Second, // Keep-alive period for an active network connection
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second, // Time spent performing the TLS handshake
		ResponseHeaderTimeout: 5 * time.Second, // Time spent reading the headers of the response
		ExpectContinueTimeout: 1 * time.Second, // Time spent waiting for the server to respond to the "100-continue" request
	},
}

type KrakenClient struct {
	APIKey        string
	APISecret     []byte
	Client        *http.Client
	ErrorLogger   *log.Logger
	autoReconnect bool

	*APIManager
	*WebSocketManager
}

// REST API
type APIManager struct {
	HandleRateLimit  atomic.Bool
	APICounter       uint8
	MaxAPICounter    uint8
	APICounterDecay  uint8 // seconds per 1 counter decay
	CounterDecayCond *sync.Cond
	Mutex            sync.Mutex
}

// WebSocket API
type WebSocketManager struct {
	WebSocketClient      *WebSocketClient
	AuthWebSocketClient  *WebSocketClient
	reconnectMgr         *ReconnectManager
	WebSocketToken       string
	SubscriptionMgr      *SubscriptionManager
	OrderBookMgr         *OrderBookManager
	TradeLogger          *TradeLogger
	OpenOrdersMgr        *OpenOrderManager
	TradingRateLimiter   *TradingRateLimiter
	LimitChaseMgr        *LimitChaseManager
	BalanceMgr           *BalanceManager
	SystemStatusCallback func(status string)
	OrderStatusCallback  func(orderStatus interface{})
	ConnectWaitGroup     *sync.WaitGroup
	ErrorLogger          *log.Logger
	Mutex                sync.RWMutex
}

type ReconnectManager struct {
	numDisconnected atomic.Int32
	mutex           sync.Mutex
	reconnectCond   *sync.Cond
}

type WebSocketClient struct {
	Conn             *websocket.Conn
	Ctx              context.Context
	Cancel           context.CancelFunc
	Router           MessageRouter
	Authenticator    Authenticator
	reconnector      *ReconnectManager
	resubscriber     resubscriber
	attemptReconnect bool
	isReconnecting   atomic.Bool
	ErrorLogger      *log.Logger
	managerWaitGroup *sync.WaitGroup // managerWaitGroup is a reference back to WebSocketManager's ConnectWaitGroup
	Mutex            sync.Mutex
}

type MessageRouter interface {
	routeMessage(msg []byte) error
}

// Authenticator is a field in WebSocketClient to hold a pointer to KrakenClient
// for access to authenticating token methods without creating circular depencies
type Authenticator interface {
	AuthenticateWebSockets() error
	reauthenticate()
}

// resubscriber is a field in WebSocketClient to hold a pointer to WebSocketManger
// for access to resubscribing to channels without creating circular depencies
type resubscriber interface {
	resubscribe(url string) error
}

// Internal order book management
type OrderBookManager struct {
	OrderBooks map[string]map[string]*InternalOrderBook
	isTracking atomic.Bool
	Mutex      sync.RWMutex
}

// Keeps track of all open subscriptions
type SubscriptionManager struct {
	PublicSubscriptions  map[string]map[string]*Subscription
	PrivateSubscriptions map[string]*Subscription
	SubscribeWaitGroup   sync.WaitGroup
	Mutex                sync.RWMutex
}

type TradeLogger struct {
	file       *os.File
	writer     *bufio.Writer
	wg         sync.WaitGroup
	ch         chan (map[string]WSOwnTrade)
	seqErrorCh chan error
	seq        int
	startSeq   int
	ctx        context.Context
	cancel     context.CancelFunc
	isLogging  atomic.Bool
}

type OpenOrderManager struct {
	OpenOrders map[string]WSOpenOrder
	ch         chan (WSOpenOrdersResp)
	seq        int
	wg         sync.WaitGroup
	isTracking atomic.Bool
	Mutex      sync.RWMutex
}

type TradingRateLimiter struct {
	HandleRateLimit      atomic.Bool
	Counters             map[string]*atomic.Int32
	UnbufferedMaxCounter int32
	MaxCounter           int32
	CounterDecay         uint16 // milliseconds per 1 counter decay
	CounterDecayConds    map[string]*sync.Cond
	Mutex                sync.Mutex
}

type LimitChaseManager struct {
	LimitChaseOrders map[int32]*LimitChase
	Mutex            sync.RWMutex
}

type BalanceManager struct {
	Balances    map[string]decimal.Decimal
	CurrencyMap map[string]BaseQuoteCurrency
	ch          chan WSOwnTrade
	isActive    bool
	ctx         context.Context
	cancel      context.CancelFunc
	mutex       sync.RWMutex
}

type BaseQuoteCurrency struct {
	Base  string
	Quote string
}

type Subscription struct {
	ChannelName         string
	Pair                string
	DataChan            chan interface{}
	DoneChan            chan struct{}
	ConfirmedChan       chan struct{}
	DataChanClosed      int32
	DoneChanClosed      int32
	ConfirmedChanClosed int32
	Callback            GenericCallback
}

type GenericCallback func(data interface{})

// Creates new authenticated client KrakenClient for Kraken API with keys passed
// to args 'apiKey' and 'apiSecret'. Constructor requires 'verificationTier', but
// this value is only used if any self rate-limiting features are activated with
// either StartRESTRateLimiter() or StartTradingRateLimiter(). Accepts up to one
// optional boolean arg passed to 'reconnect' which will cause client to attempt
// reconnect, reauthenticate (if applicable), and resubscribe to all WebSocket
// channels if connection is lost. Defaults to true if not passed.
//
// Verification Tiers:
//
// 1. Starter
//
// 2. Intermediate
//
// 3. Pro
//
// # Enum:
//
// 'verificationTier': [1..3]
func NewKrakenClient(apiKey, apiSecret string, verificationTier uint8, autoReconnect ...bool) (*KrakenClient, error) {
	if len(autoReconnect) > 1 {
		return nil, fmt.Errorf("%w; expected 3 or 4", ErrTooManyArgs)
	}
	recon := true
	if len(autoReconnect) > 0 {
		recon = autoReconnect[0]
	}
	decodedSecret, err := base64.StdEncoding.DecodeString(apiSecret)
	if err != nil {
		return nil, err
	}
	decayRate, ok := decayRateMap[verificationTier]
	if !ok {
		err = fmt.Errorf("invalid verification tier, check enum and inputs and try again")
		return nil, err
	}
	maxCounter := maxCounterMap[verificationTier]
	logger := log.New(os.Stderr, "", log.LstdFlags)
	kc := &KrakenClient{
		APIKey:        apiKey,
		APISecret:     decodedSecret,
		Client:        sharedClient,
		ErrorLogger:   logger,
		autoReconnect: recon,
	}
	kc.APIManager = &APIManager{
		HandleRateLimit: atomic.Bool{},
		MaxAPICounter:   maxCounter,
		APICounterDecay: decayRate,
	}
	kc.APIManager.CounterDecayCond = sync.NewCond(&kc.APIManager.Mutex)

	kc.APIManager.HandleRateLimit.Store(false)
	maxTradingCounter := maxTradingCounterMap[verificationTier]
	decayTradingRate := decayTradingRateMap[verificationTier]
	kc.WebSocketManager = &WebSocketManager{
		SubscriptionMgr: &SubscriptionManager{
			PublicSubscriptions:  make(map[string]map[string]*Subscription),
			PrivateSubscriptions: make(map[string]*Subscription),
		},
		OrderBookMgr: &OrderBookManager{
			OrderBooks: make(map[string]map[string]*InternalOrderBook),
			isTracking: atomic.Bool{},
		},
		TradingRateLimiter: &TradingRateLimiter{
			Counters:             make(map[string]*atomic.Int32),
			UnbufferedMaxCounter: int32(maxTradingCounter),
			MaxCounter:           int32(maxTradingCounter),
			CounterDecay:         decayTradingRate,
			HandleRateLimit:      atomic.Bool{},
		},
		ErrorLogger: logger,
		LimitChaseMgr: &LimitChaseManager{
			LimitChaseOrders: make(map[int32]*LimitChase),
		},
		BalanceMgr: &BalanceManager{
			isActive: false,
		},
		ConnectWaitGroup: &sync.WaitGroup{},
		reconnectMgr: &ReconnectManager{
			numDisconnected: atomic.Int32{},
			mutex:           sync.Mutex{},
		},
	}
	kc.WebSocketManager.reconnectMgr.reconnectCond = sync.NewCond(&kc.WebSocketManager.reconnectMgr.mutex)
	kc.OrderBookMgr.isTracking.Store(false)
	kc.TradingRateLimiter.HandleRateLimit.Store(false)

	return kc, nil
}

// SetErrorLogger creates a new custom error logger for KrakenClient and its
// components. The logger logs to the provided 'output' io.Writer. This method
// also sets the created logger as the ErrorLogger for the KrakenClient and its
// WebSocketManager and WebSocketClients. It returns the newly created logger.
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
//	// Create a new KrakenClient
//	kc := krakenspot.NewKrakenClient()
//
//	// Set the KrakenClient logger to write to the file
//	logger := kc.SetErrorLogger(file)
//
//	// Now when you use the KrakenClient, it will log internal errors to the file
//	// But you still need to call the logger for externally returned errors
//	err = kc.Connect()
//	if err != nil {
//		logger.Println("error connecting")
//	}
func (kc *KrakenClient) SetErrorLogger(output io.Writer) *log.Logger {
	logger := log.New(output, "", log.LstdFlags)
	kc.ErrorLogger = logger
	kc.WebSocketManager.ErrorLogger = logger
	if kc.WebSocketManager.WebSocketClient != nil {
		kc.WebSocketManager.WebSocketClient.ErrorLogger = logger
	}
	if kc.WebSocketManager.AuthWebSocketClient != nil {
		kc.WebSocketManager.AuthWebSocketClient.ErrorLogger = logger
	}
	return logger
}
