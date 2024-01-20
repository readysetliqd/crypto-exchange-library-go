package krakenspot

import (
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

	// WebSockets fields
	*WebSocketManager

	// General API self rate limiting fields
	HandleRateLimit  bool
	APICounter       uint8
	MaxAPICounter    uint8
	APICounterDecay  uint8
	Mutex            sync.Mutex
	CounterDecayCond *sync.Cond
}

type WebSocketManager struct {
	WebSocketToken  string
	WebSocketClient *websocket.Conn
	Mutex           sync.RWMutex
	SubscriptionMgr *SubscriptionManager
	OrderBookMgr    *OrderBookManager
}

type OrderBookManager struct {
	OrderBooks map[string]map[string]*InternalOrderBook
	Mutex      sync.RWMutex
}

type SubscriptionManager struct {
	PublicSubscriptions  map[string]map[string]*Subscription
	PrivateSubscriptions map[string]*Subscription
	Mutex                sync.RWMutex
}

type Subscription struct {
	ChannelName    string
	Pair           string
	DataChanClosed int32
	DoneChanClosed int32
	DataChan       chan interface{}
	DoneChan       chan struct{}
	Callback       GenericCallback
	ConfirmedChan  chan struct{}
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
		APIKey:          apiKey,
		APISecret:       decodedSecret,
		Client:          sharedClient,
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
		kc.CounterDecayCond = sync.NewCond(&kc.Mutex)
	}
	return kc, nil
}

// A go func method with a timer that decays kc.APICounter every second by the
// amount in kc.APICounterDecay. Stops at 0.
func (kc *KrakenClient) startRateLimiter() {
	ticker := time.NewTicker(time.Second * time.Duration(kc.APICounterDecay))
	defer ticker.Stop()
	for range ticker.C {
		kc.Mutex.Lock()
		if kc.APICounter > 0 {
			kc.APICounter -= 1
		}
		kc.Mutex.Unlock()
		kc.CounterDecayCond.Broadcast()
	}
}
