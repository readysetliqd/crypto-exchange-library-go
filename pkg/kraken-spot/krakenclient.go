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
	APIKey      string
	APISecret   []byte
	Client      *http.Client
	ErrorLogger *log.Logger

	*APIManager
	*WebSocketManager
	*StateManager
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
	WebSocketToken       string
	SubscriptionMgr      *SubscriptionManager
	OrderBookMgr         *OrderBookManager
	TradeLogger          *TradeLogger
	OpenOrdersMgr        *OpenOrderManager
	TradingRateLimiter   *TradingRateLimiter
	SystemStatusCallback func(status string)
	OrderStatusCallback  func(orderStatus interface{})
	ErrorLogger          *log.Logger
	Mutex                sync.RWMutex
}

type WebSocketClient struct {
	Conn           *websocket.Conn
	Ctx            context.Context
	Cancel         context.CancelFunc
	Router         MessageRouter
	Authenticator  Authenticator
	IsReconnecting atomic.Bool
	ErrorLogger    *log.Logger
	Mutex          sync.Mutex
}

type MessageRouter interface {
	routeMessage(msg []byte) error
}

type Authenticator interface {
	AuthenticateWebSockets() error
	reauthenticate()
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

type StateManager struct {
	states       map[string]State
	currentState State
	Mutex        sync.RWMutex
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

// TODO remove handleratelimit and docstrings referring to it
// Creates new authenticated client KrakenClient for Kraken API with keys passed
// to args 'apiKey' and 'apiSecret'. Constructor requires 'verificationTier', but
// this value is only used if any self rate-limiting features are activated with
// either StartRESTRateLimiter() or StartTradingRateLimiter().
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
func NewKrakenClient(apiKey, apiSecret string, verificationTier uint8) (*KrakenClient, error) {
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
		APIKey:      apiKey,
		APISecret:   decodedSecret,
		Client:      sharedClient,
		ErrorLogger: logger,
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
	}
	kc.OrderBookMgr.isTracking.Store(false)
	kc.TradingRateLimiter.HandleRateLimit.Store(false)

	return kc, nil
}

// Creates new custom error logger for KrakenClient and its components to log
// to file provided to 'output'. This method returns the newly created logger.
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
