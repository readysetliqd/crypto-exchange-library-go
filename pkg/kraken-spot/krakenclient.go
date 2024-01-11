package krakenspot

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var sharedClient = &http.Client{}

type KrakenClient struct {
	APIKey          string
	APISecret       []byte
	Client          *http.Client
	WebSocketsToken string
	HandleRateLimit bool
	APICounter      uint8
	MaxAPICounter   uint8
	APICounterDecay uint8
	Mutex           sync.Mutex
	Cond            *sync.Cond
}

// TODO document inputs and enums
// Creates new authenticated client KrakenClient for Kraken API
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
	if handleRateLimit {
		go kc.startRateLimiter()
		kc.Cond = sync.NewCond(&kc.Mutex)
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
		kc.Cond.Broadcast()
	}
}
