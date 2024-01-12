package krakenspot

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/readysetliqd/crypto-exchange-library-go/pkg/kraken-spot/internal/data"
)

func (kc *KrakenClient) SubscribeToTicker(pairs []string) (map[string]<-chan data.WSTickerResp, error) {
	err := kc.connectAndMaintain()
	if err != nil {
		err = fmt.Errorf("error calling connectandmaintain() | %w", err)
		return nil, err
	}

	kc.WebSocketsMutex.Lock()
	defer kc.WebSocketsMutex.Unlock()

	channels := make(map[string]<-chan data.WSTickerResp)
	for _, pair := range pairs {
		tickerChan := make(chan data.WSTickerResp)
		kc.TickerChannels[pair] = tickerChan
		channels[pair] = tickerChan

		go func(pair string, tickerChan chan data.WSTickerResp) {
			for {
				var message data.WSTickerResp
				err := kc.WebSocketClient.ReadJSON(&message)
				if err != nil {
					close(tickerChan)
					return
				}
				tickerChan <- message
			}
		}(pair, tickerChan)
	}

	// send subscription message
	payload := fmt.Sprintf(`{
		"event": "subscribe",
		"pair": ["%s"],
		"subscription": {
			"name": "ticker"
		}
	}`, strings.Join(pairs, `","`))

	err = kc.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing subscription message | %w", err)
		return nil, err
	}
	var initResponse data.WSSubscription
	err = kc.WebSocketClient.ReadJSON(&initResponse)
	if err != nil {
		err = fmt.Errorf("error reading json | %w", err)
		return nil, err
	}

	return channels, nil
}

func (kc *KrakenClient) connectAndMaintain() error {
	kc.WebSocketsMutex.Lock()
	defer kc.WebSocketsMutex.Unlock()

	if kc.WebSocketClient == nil {
		err := kc.dialKraken()
		if err != nil {
			return err
		}
		// Start maintaining the connection in a separate goroutine
		go kc.maintainConnection()
	}
	return nil
}

func (kc *KrakenClient) maintainConnection() {
	// Set a read deadline for the connection
	kc.WebSocketClient.SetReadDeadline(time.Now().Add(wsTimeoutDuration))

	for {
		_, _, err := kc.WebSocketClient.ReadMessage()
		if err != nil {
			// Check if the error is due to a timeout
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// No messages received within the timeout duration, reconnect
				kc.reconnect()
			} else {
				// Other errors, handle accordingly
			}
			continue
		}

		// Reset the read deadline upon receiving a message
		kc.WebSocketClient.SetReadDeadline(time.Now().Add(wsTimeoutDuration))
	}
}

// TODO implement reconnection logic recursive?
func (kc *KrakenClient) reconnect() error {
	err := kc.dialKraken()
	if err != nil {
		return err
	}
	return nil
}

// TODO do something about listening for system status messages at other times besides connection
func (kc *KrakenClient) dialKraken() error {
	conn, _, err := websocket.DefaultDialer.Dial(wsPrivateURL, http.Header{})
	if err != nil {
		err = fmt.Errorf("error dialing kraken | %w", err)
		return err
	}
	kc.WebSocketClient = conn

	var initResponse data.WSConnection
	err = kc.WebSocketClient.ReadJSON(&initResponse)
	if !(initResponse.Event == "systemStatus" && initResponse.Status == "online") {
		err = fmt.Errorf("error establishing websockets connection. system sattus | %w", initResponse.Status)
		return err
	}
	return nil
}
