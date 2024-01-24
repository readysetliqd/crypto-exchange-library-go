package krakenspot

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"sync"
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
	APIKey    string
	APISecret []byte
	Client    *http.Client

	*APIManager
	*WebSocketManager
}

// REST API
type APIManager struct {
	HandleRateLimit  bool
	APICounter       uint8
	MaxAPICounter    uint8
	APICounterDecay  uint8
	CounterDecayCond *sync.Cond
	Mutex            sync.Mutex
}

// WebSocket API
type WebSocketManager struct {
	WebSocketClient         *websocket.Conn
	WebSocketCtx            context.Context
	WebSocketCancel         context.CancelFunc
	WebSocketWriteMutex     sync.Mutex
	AuthWebSocketClient     *websocket.Conn
	AuthWebSocketCtx        context.Context
	AuthWebSocketCancel     context.CancelFunc
	AuthWebSocketWriteMutex sync.Mutex
	WebSocketToken          string
	SubscriptionMgr         *SubscriptionManager
	OrderBookMgr            *OrderBookManager
	SystemStatusCallback    func(status string)
	OrderStatusCallback     func(orderStatus interface{})
	Mutex                   sync.RWMutex
}

type WebSocketClient struct {
	Conn   *websocket.Conn
	Ctx    context.Context
	Cancel context.CancelFunc
	Mutex  sync.Mutex
}

// Internal order book management
type OrderBookManager struct {
	OrderBooks map[string]map[string]*InternalOrderBook
	Mutex      sync.RWMutex
}

// Keeps track of all open subscriptions
type SubscriptionManager struct {
	PublicSubscriptions  map[string]map[string]*Subscription
	PrivateSubscriptions map[string]*Subscription
	Mutex                sync.RWMutex
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
// to args 'apiKey' and 'apiSecret'. Constructor requires 'verificationTier' but
// is only used if 'handleRateLimit' is set to "true". Arg 'handleRateLimit' will
// allow the client to self rate limit for general API calls. This currently has no
// ability to rate limit Trading endpoint calls (such as AddOrder(), EditOrder(),
// or CancelOrder()). This feature adds processing overhead so it should be set to
// "false" if many consecutive general API calls won't be made, or if the application
// importing this package is either performance critical or handling rate limiting
// itself.
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
//
// 'handleRateLimit': true, false
func NewKrakenClient(apiKey, apiSecret string, verificationTier uint8, handleRateLimit bool) (*KrakenClient, error) {
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
	kc := &KrakenClient{
		APIKey:    apiKey,
		APISecret: decodedSecret,
		Client:    sharedClient,
	}
	kc.APIManager = &APIManager{
		HandleRateLimit: handleRateLimit,
		MaxAPICounter:   maxCounter,
		APICounterDecay: decayRate,
	}
	kc.WebSocketManager = &WebSocketManager{
		SubscriptionMgr: &SubscriptionManager{
			PublicSubscriptions:  make(map[string]map[string]*Subscription),
			PrivateSubscriptions: make(map[string]*Subscription),
		},
		OrderBookMgr: &OrderBookManager{
			OrderBooks: make(map[string]map[string]*InternalOrderBook),
		},
	}
	if handleRateLimit {
		go kc.startRateLimiter()
		kc.APIManager.CounterDecayCond = sync.NewCond(&kc.APIManager.Mutex)
	}
	return kc, nil
}

// A go routine method with a timer that decays kc.APICounter every second by the
// amount in kc.APICounterDecay. Stops at 0.
func (kc *KrakenClient) startRateLimiter() {
	ticker := time.NewTicker(time.Second * time.Duration(kc.APICounterDecay))
	defer ticker.Stop()
	for range ticker.C {
		kc.APIManager.Mutex.Lock()
		if kc.APICounter > 0 {
			kc.APICounter -= 1
			if kc.APICounter == 0 {
				ticker.Stop()
			}
		}
		kc.APIManager.Mutex.Unlock()
		kc.CounterDecayCond.Broadcast()
	}
}
