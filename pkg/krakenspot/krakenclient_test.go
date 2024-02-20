package krakenspot

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"
)

func TestNewKrakenClient(t *testing.T) {
	// Test with valid arguments
	apiKeyStr := "test-api-key"
	apiSecretStr := "test-api-secret"
	// Encode the string to base64.
	apiKey := base64.StdEncoding.EncodeToString([]byte(apiKeyStr))
	apiSecret := base64.StdEncoding.EncodeToString([]byte(apiSecretStr))
	verificationTier := uint8(2)
	client, err := NewKrakenClient(apiKey, apiSecret, verificationTier)
	if err != nil {
		t.Errorf("NewKrakenClient() with valid arguments returned error: %v", err)
	}
	if client.APIKey != apiKey {
		t.Errorf("NewKrakenClient() didn't set APIKey correctly, got: %v, want: %v", client.APIKey, apiKey)
	}

	// Test with too many arguments
	_, err = NewKrakenClient(apiKey, apiSecret, verificationTier, true, false)
	if err == nil {
		t.Error("NewKrakenClient() with too many arguments didn't return an error")
	}

	// Test with invalid verification tier
	_, err = NewKrakenClient(apiKey, apiSecret, 99)
	if err == nil {
		t.Error("NewKrakenClient() with invalid verification tier didn't return an error")
	}

	// Test with invalid API secret
	_, err = NewKrakenClient(apiKey, "invalid-api-secret", verificationTier)
	if err == nil {
		t.Error("NewKrakenClient() with invalid API secret didn't return an error")
	}
}

func TestSetErrorLogger(t *testing.T) {
	// Test with valid arguments
	apiKeyStr := "test-api-key"
	apiSecretStr := "test-api-secret"
	// Encode the string to base64.
	apiKey := base64.StdEncoding.EncodeToString([]byte(apiKeyStr))
	apiSecret := base64.StdEncoding.EncodeToString([]byte(apiSecretStr))

	verificationTier := uint8(2)
	client, _ := NewKrakenClient(apiKey, apiSecret, verificationTier)

	// Test with valid arguments
	buf := new(bytes.Buffer)
	logger := client.SetErrorLogger(buf)
	logger.Println("test log message")
	if buf.String() == "" {
		t.Error("SetErrorLogger() didn't set the logger correctly")
	}

	// Test that the logger was set on the WebSocketManager and WebSocketClients
	client.WebSocketManager.ErrorLogger.Println("test log message")
	if buf.String() == "" {
		t.Error("SetErrorLogger() didn't set the logger on the WebSocketManager")
	}
	client.WebSocketManager.WebSocketClient = &WebSocketClient{Ctx: context.Background()}
	client.WebSocketManager.AuthWebSocketClient = &WebSocketClient{Ctx: context.Background()}
	client.SetErrorLogger(buf)
	client.WebSocketManager.WebSocketClient.ErrorLogger.Println("test log message")
	client.WebSocketManager.AuthWebSocketClient.ErrorLogger.Println("test log message")
	if buf.String() == "" {
		t.Error("SetErrorLogger() didn't set the logger on the WebSocketClients")
	}
}
