package krakenspot

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/readysetliqd/crypto-exchange-library-go/pkg/kraken-spot/internal/data"
)

var sharedClient = &http.Client{}

type KrakenClient struct {
	APIKey    string
	APISecret []byte
	Client    *http.Client

	// WebSockets fields
	WebSocketsToken string
	WebSocketClient *websocket.Conn
	WebSocketsMutex sync.RWMutex
	TickerChannels  map[string]chan data.WSTickerResp

	// General API self rate limiting fields
	HandleRateLimit  bool
	APICounter       uint8
	MaxAPICounter    uint8
	APICounterDecay  uint8
	APIMutex         sync.Mutex
	CounterDecayCond *sync.Cond
}

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
	kc.TickerChannels = make(map[string]chan data.WSTickerResp)
	if handleRateLimit {
		go kc.startRateLimiter()
		kc.CounterDecayCond = sync.NewCond(&kc.APIMutex)
	}
	return kc, nil
}

// A go func method with a timer that decays kc.APICounter every second by the
// amount in kc.APICounterDecay. Stops at 0.
func (kc *KrakenClient) startRateLimiter() {
	ticker := time.NewTicker(time.Second * time.Duration(kc.APICounterDecay))
	defer ticker.Stop()
	for range ticker.C {
		kc.APIMutex.Lock()
		if kc.APICounter > 0 {
			kc.APICounter -= 1
		}
		kc.APIMutex.Unlock()
		kc.CounterDecayCond.Broadcast()
	}
}
