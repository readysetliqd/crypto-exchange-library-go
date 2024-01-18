package krakenspot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

// #region Exported *KrakenClient and *WebSocketManager methods (Connect Subscribe<> and Unsubscribe<>)

// Creates authenticated connection to Kraken WebSocket server.
//
// Note: Creates authenticated token which expires within 15 minutes. If
// private-data channel subscription is desired, recommened subscribing to at
// least one private-data channel before token expiry (ownTrades or openOrders)
func (kc *KrakenClient) Connect() error {
	kc.AuthenticateWebSockets()
	kc.dialKraken()
	kc.startMessageReader()
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
func (kc *KrakenClient) UnsubscribeTicker(pair string) error {
	channelName := "ticker"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
	err := kc.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
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
func (kc *KrakenClient) UnsubscribeOHLC(pair string, interval uint16) error {
	channelName := "ohlc"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s", "interval": %v}}`, pair, channelName, interval)
	err := kc.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing message | %w", err)
		return err
	}
	return nil
}

// TODO write docstrings with example usage
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
//	err = kc.SubscribeTrades("XBT/USD", tradeCallback)
//	if err != nil {
//		log.Println(err)
//	}
func (ws *WebSocketManager) SubscribeTrades(pair string, callback GenericCallback) error {
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
//	err := kc.UnsubscribeTrades("XBT/USD")
//	if err != nil...
func (kc *KrakenClient) UnsubscribeTrades(pair string) error {
	channelName := "trade"
	payload := fmt.Sprintf(`{"event": "unsubscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
	err := kc.WebSocketClient.WriteMessage(websocket.TextMessage, []byte(payload))
	if err != nil {
		err = fmt.Errorf("error writing message | %w", err)
		return err
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
	switch {
	case msg.ChannelName == "ticker":
		tickerMsg, ok := msg.Content.(WSTickerResp)
		if !ok {
			return fmt.Errorf("error asserting msg.content to wstickerresp type")
		}
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[tickerMsg.ChannelName][tickerMsg.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[tickerMsg.ChannelName][tickerMsg.Pair].DataChan <- tickerMsg
		}
	case strings.HasPrefix(msg.ChannelName, "ohlc"):
		ohlcMsg, ok := msg.Content.(WSOHLCResp)
		if !ok {
			return fmt.Errorf("error asserting msg.content to wsohlcresp type")
		}
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[ohlcMsg.ChannelName][ohlcMsg.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[ohlcMsg.ChannelName][ohlcMsg.Pair].DataChan <- ohlcMsg
		}
	case msg.ChannelName == "trade":
		tradeMsg, ok := msg.Content.(WSTradeResp)
		if !ok {
			return fmt.Errorf("error asserting msg.content to wstraderesp type")
		}
		// send to channel if open
		if ws.SubscriptionMgr.PublicSubscriptions[tradeMsg.ChannelName][tradeMsg.Pair].DataChanClosed == 0 {
			ws.SubscriptionMgr.PublicSubscriptions[tradeMsg.ChannelName][tradeMsg.Pair].DataChan <- tradeMsg
		}
	case msg.ChannelName == "spread":
	case strings.HasPrefix(msg.ChannelName, "book"):
	default:
		return fmt.Errorf("cannot route unknown channel name | %s", msg.ChannelName)
	}
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routePrivateMessage(msg *GenericArrayMessage) error {
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routeOrderMessage(msg *GenericMessage) error {
	return nil
}

// Asserts message to correct unique data type and routes to the appropriate
// channel if channel is still open.
func (ws *WebSocketManager) routeGeneralMessage(msg *GenericMessage) error {
	switch msg.Event {
	case "subscriptionStatus":
		if subscriptionStatusMsg, ok := msg.Content.(WSSubscriptionStatus); !ok {
			return fmt.Errorf("error asserting msg.content to data.wssubscriptionstatus type")
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
				} else if privateChannelNames[subscriptionStatusMsg.ChannelName] {
					ws.SubscriptionMgr.PrivateSubscriptions[subscriptionStatusMsg.ChannelName].unsubscribe()
				}
			case "error":
				return fmt.Errorf("subscribe/unsubscribe error msg received. operation not completed; check inputs and try again | %s", subscriptionStatusMsg.ErrorMessage)
			default:
				return fmt.Errorf("cannot route unknown subscriptionStatus status | %s", subscriptionStatusMsg.Status)
			}
		}
	// TODO add other event types, pong, systemStatus, ???
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
	var initResponse WSConnection
	err = ws.WebSocketClient.ReadJSON(&initResponse)
	if err != nil {
		err = fmt.Errorf("error reading json | %w", err)
		return err
	}
	if !(initResponse.Event == "systemStatus" && initResponse.Status == "online") {
		err = fmt.Errorf("error establishing websockets connection. system status | %s", initResponse.Status)
		return err
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
