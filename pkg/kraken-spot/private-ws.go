package krakenspot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

func (kc *KrakenClient) Connect() error {
	kc.AuthenticateWebSockets()
	kc.dialKraken()
	kc.startMessageReader()
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
	switch msg.ChannelName {
	// TODO add remaining channel types
	case "ticker":
		if tickerMsg, ok := msg.Content.(WSTickerResp); !ok {
			return fmt.Errorf("error asserting msg.content to data.wstickerresp type")
		} else {
			// send to channel if open
			if ws.SubscriptionMgr.PublicSubscriptions[tickerMsg.ChannelName][tickerMsg.Pair].DataChanClosed == 0 {
				ws.SubscriptionMgr.PublicSubscriptions[tickerMsg.ChannelName][tickerMsg.Pair].DataChan <- tickerMsg
			}
		}
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
	default:
		return fmt.Errorf("cannot route unknown event type | %s", msg.Event)
	}
	return nil
}

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

// Subscribes to "ticker" WebSocket channel for arg 'pair'
func (ws *WebSocketManager) SubscribeTicker(pair string, callback GenericCallback) error {
	channelName := "ticker"
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
	payload := fmt.Sprintf(`{"event": "subscribe", "pair": ["%s"], "subscription": {"name": "%s"}}`, pair, channelName)
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

// Unsubscribes from "ticker" WebSocket channel for arg 'pair'
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
