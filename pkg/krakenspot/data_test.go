package krakenspot

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestTickerBookInfo_UnmarshalJSON(t *testing.T) {
	// Test valid input
	validInput := `["1000", "2000", "3000"]`
	ti := &TickerBookInfo{}
	err := json.Unmarshal([]byte(validInput), ti)
	if err != nil {
		t.Errorf("UnmarshalJSON() returned an error for valid input: %v", err)
	}
	if ti.Price != "1000" || ti.WholeLotVolume != "2000" || ti.LotVolume != "3000" {
		t.Errorf("UnmarshalJSON() didn't correctly unmarshal valid input")
	}

	// Test invalid input (not a JSON array)
	invalidInput := `"not an array"`
	err = json.Unmarshal([]byte(invalidInput), ti)
	if err == nil {
		t.Errorf("UnmarshalJSON() didn't return an error for invalid input")
	}

	// Test unexpected field type (array of integers instead of strings)
	unexpectedFieldType := `[1, 2, 3]`
	err = json.Unmarshal([]byte(unexpectedFieldType), ti)
	if err == nil {
		t.Errorf("UnmarshalJSON() didn't return an error for unexpected field type")
	}
}

func TestTickerLastTradeInfo_UnmarshalJSON(t *testing.T) {
	var got interface{}
	var want interface{}
	var err error

	t.Run("valid input", func(t *testing.T) {
		validInput := `["30303.20000","0.00067643"]`
		var tickerLastTrade TickerLastTradeInfo
		err = json.Unmarshal([]byte(validInput), &tickerLastTrade)
		if err != nil {
			t.Errorf("UnmasrhalJSON() returned err | got: %v, want: nil", err)
		}
		got = tickerLastTrade.Price
		want = "30303.20000"
		if got != want {
			t.Errorf("UnmasrhalJSON() returned incorrect Price | got: %v, want: %v", got, want)
		}
		got = tickerLastTrade.LotVolume
		want = "0.00067643"
		if got != want {
			t.Errorf("UnmasrhalJSON() returned incorrect LotVolume | got: %v, want: %v", got, want)
		}
	})

	t.Run("wrong length", func(t *testing.T) {
		invalidLength := `["30300.00000","1","1.000", "1.000"]`
		var tickerLastTrade TickerLastTradeInfo
		err = json.Unmarshal([]byte(invalidLength), &tickerLastTrade)
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmasrhalJSON() err did not contain expected error | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})

	t.Run("invalid field type", func(t *testing.T) {
		invalidFieldType := `[1, 2, 3]`
		var tickerLastTrade TickerLastTradeInfo
		err = json.Unmarshal([]byte(invalidFieldType), &tickerLastTrade)
		if err == nil {
			t.Error("UnmarshalJSON() did not return error for invalid length | got: nil, want: non-nil")
		}
	})

	t.Run("not an array", func(t *testing.T) {
		invalidInput := "not an array"
		var tickerLastTrade TickerLastTradeInfo
		err = json.Unmarshal([]byte(invalidInput), &tickerLastTrade)
		if err == nil {
			t.Errorf("UnmarshalJSON() didn't return error for invalid input | got: nil, want: non-nil")
		}
	})
}

func TestTickerDailyInfo_UnmarshalJSON(t *testing.T) {
	var err error
	var got interface{}
	var want interface{}
	var tickerInfo TickerDailyInfo

	t.Run("valid input", func(t *testing.T) {
		validInput := `["12.00","1.0123"]`
		err = json.Unmarshal([]byte(validInput), &tickerInfo)
		if err != nil {
			t.Errorf("UnmarshalJSON() returned err | got: %v, want: nil", err)
		}
		got = tickerInfo.Today
		want = "12.00"
		if got != want {
			t.Errorf("UnmarshalJSON() didn't store correct value in field 'Today' | got: %v, want: %v", got, want)
		}
		got = tickerInfo.Last24Hours
		want = "1.0123"
		if got != want {
			t.Errorf("UnmarshalJSON() didn't store correct value in field 'Last24Hours' | got: %v, want: %v", got, want)
		}
	})

	t.Run("not an array", func(t *testing.T) {
		invalidInput := `{"12.00", "1.0123"}`
		err = json.Unmarshal([]byte(invalidInput), &tickerInfo)
		if err == nil {
			t.Errorf("UnmarshalJSON() didn't return err | got: nil, want: non-nil")
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		invalidLength := `["12.00", "1.0123", "4"]`
		err = json.Unmarshal([]byte(invalidLength), &tickerInfo)
		if err == nil {
			t.Errorf("UnmarshalJSON() didn't return err | got: nil, want: non-nil")
		}
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmarshalJSON() didn't wrap correct err | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})

	t.Run("invalid field type", func(t *testing.T) {
		invalidInput := "[1, 2, 3]"
		err = json.Unmarshal([]byte(invalidInput), &tickerInfo)
		if err == nil {
			t.Errorf("UnmarshalJSON didn't return err for invalid field type | got: nil, want: non-nil")
		}
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmarshalJSON didn't return correct error | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})
}

func TestTickerDailyInfoInt_UnmarshalJSON(t *testing.T) {
	var err error
	var got interface{}
	var want interface{}
	var tickerInfo TickerDailyInfoInt

	t.Run("valid input", func(t *testing.T) {
		validInput := `[12, 2]`
		err = json.Unmarshal([]byte(validInput), &tickerInfo)
		if err != nil {
			t.Errorf("UnmarshalJSON() returned err | got: %v, want: nil", err)
		}
		got = tickerInfo.Today
		want = 12
		if got != want {
			t.Errorf("UnmarshalJSON() didn't store correct value in field 'Today' | got: %v, want: %v", got, want)
		}
		got = tickerInfo.Last24Hours
		want = 2
		if got != want {
			t.Errorf("UnmarshalJSON() didn't store correct value in field 'Last24Hours' | got: %v, want: %v", got, want)
		}
	})

	t.Run("not an array", func(t *testing.T) {
		invalidInput := `{12, 1}`
		err = json.Unmarshal([]byte(invalidInput), &tickerInfo)
		if err == nil {
			t.Errorf("UnmarshalJSON() didn't return err | got: nil, want: non-nil")
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		invalidLength := `[12, 1, 4]`
		err = json.Unmarshal([]byte(invalidLength), &tickerInfo)
		if err == nil {
			t.Errorf("UnmarshalJSON() didn't return err | got: nil, want: non-nil")
		}
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmarshalJSON() didn't wrap correct err | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})

	t.Run("invalid field type", func(t *testing.T) {
		invalidInput := `["1", "2", "3"]`
		err = json.Unmarshal([]byte(invalidInput), &tickerInfo)
		if err == nil {
			t.Errorf("UnmarshalJSON didn't return err for invalid field type | got: nil, want: non-nil")
		}
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmarshalJSON didn't return correct error | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})
}

func TestOHLCResp_UnmarshalJSON(t *testing.T) {
	var got interface{}
	var want interface{}
	var err error
	t.Run("valid input", func(t *testing.T) {
		validInput := `{
			"XXBTZUSD":[
			[1688671200,"30306.1","30306.2","30305.7","30305.7","30306.1","3.39243896",23],
			[1688671260,"30304.5","30304.5","30300.0","30300.0","30300.0","4.42996871",18]
			], 
		"last":1688672160}`
		ohlcResp := OHLCResp{}
		err = json.Unmarshal([]byte(validInput), &ohlcResp)
		if err != nil {
			t.Errorf("UnmarshalJSON() error on valid input | got: %v, want: nil", err)
		}
		got = len(ohlcResp.Data)
		// Even though there's two slices, one gets unmarshalled into Current
		want = 1
		if got != want {
			t.Errorf("UnmarshalJSON() did not unmarshal correct length of data | got: %v, want: %v", got, want)
		}
		got = ohlcResp.Last
		want = uint64(1688672160)
		if got != want {
			t.Errorf("UnmarshalJSON() didn't unmarshal Last correctly | got: %v, want: %v", got, want)
		}
		got = ohlcResp.Ticker
		want = "XXBTZUSD"
		if got != want {
			t.Errorf("UnmarshalJSON() didn't unmarshal Ticker correctly | got: %v, want: %v", got, want)
		}
		got = ohlcResp.Current.Close
		want = "30300.0"
		if got != want {
			t.Errorf("UnmarshalJSON() didn't unmarshal Current.Close correctly | got: %v, want: %v", got, want)
		}
		got = ohlcResp.Data[0].Time
		want = uint64(1688671200)
		if got != want {
			t.Errorf("UnmarshalJSON() didn't unmarshal ohlcResp.Data[0].Time correctly | got: %v, want: %v", got, want)
		}
		got = ohlcResp.Data[0].Count
		want = uint32(23)
		if got != want {
			t.Errorf("UnmarshalJSON() didn't unmarshal ohlcResp.Data[0].Count correctly | got: %v, want: %v", got, want)
		}
	})

	t.Run("not a json object", func(t *testing.T) {
		invalidInput := "not a json object"
		ohlcResp := OHLCResp{}
		err = json.Unmarshal([]byte(invalidInput), &ohlcResp)
		if err == nil {
			t.Errorf("UnmarshalJSON() nil error on invalid input | got: nil, want: non-nil")
		}
	})

	t.Run("invalid last field type", func(t *testing.T) {
		invalidField := `{
			"XXBTZUSD":[
			[1688671200,"30306.1","30306.2","30305.7","30305.7","30306.1","3.39243896",23],
			[1688671260,"30304.5","30304.5","30300.0","30300.0","30300.0","4.42996871",18]
			], 
		"last":"1688672160"}`
		ohlcResp := OHLCResp{}
		err = json.Unmarshal([]byte(invalidField), &ohlcResp)
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmarshalJSON() didn't throw expected error on invalid input | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})

	t.Run("invalid key type", func(t *testing.T) {
		invalidField := `{
			135123:[
			[1688671200,"30306.1","30306.2","30305.7","30305.7","30306.1","3.39243896",23],
			[1688671260,"30304.5","30304.5","30300.0","30300.0","30300.0","4.42996871",18]
			], 
		"last":1688672160}`
		ohlcResp := OHLCResp{}
		err = json.Unmarshal([]byte(invalidField), &ohlcResp)
		if err == nil {
			t.Errorf("UnmarshalJSON() nil error on invalid input | got: nil, want: non-nil")
		}
	})

	t.Run("invalid data slice field type", func(t *testing.T) {
		invalidField := `{
			"XXBTZUSD":[
			["1688671200","30306.1","30306.2","30305.7","30305.7","30306.1","3.39243896",23],
			["1688671260","30304.5","30304.5","30300.0","30300.0","30300.0","4.42996871",18]
			], 
		"last":1688672160}`
		ohlcResp := OHLCResp{}
		err = json.Unmarshal([]byte(invalidField), &ohlcResp)
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmarshalJSON() didn't throw expected error on invalid input | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})

	t.Run("invalid data slice field type", func(t *testing.T) {
		invalidField := `{
			"XXBTZUSD":[
			["1688671200","30306.1","30306.2","30305.7","30305.7","30306.1","3.39243896",23],
			["1688671260","30304.5","30304.5","30300.0","30300.0","30300.0","4.42996871",18]
			], 
		"last":1688672160}`
		ohlcResp := OHLCResp{}
		err = json.Unmarshal([]byte(invalidField), &ohlcResp)
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmarshalJSON() didn't throw expected error on invalid input | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})

	t.Run("invalid data slice length", func(t *testing.T) {
		invalidField := `{
			"XXBTZUSD":[
			[1688671200,"30306.1","30306.2","30305.7","30305.7","30306.1","3.39243896"],
			[1688671260,"30304.5","30304.5","30300.0","30300.0","30300.0","4.42996871"]
			], 
		"last":1688672160}`
		ohlcResp := OHLCResp{}
		err = json.Unmarshal([]byte(invalidField), &ohlcResp)
		if !errors.Is(err, ErrUnexpectedJSONInput) {
			t.Errorf("UnmarshalJSON() didn't throw expected error on invalid input | got: %v, want: %v", err, ErrUnexpectedJSONInput)
		}
	})
}
