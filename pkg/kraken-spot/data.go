package krakenspot

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/shopspring/decimal"
)

type ApiResp struct {
	Error  []string    `json:"error"`
	Result interface{} `json:"result"`
}

// #region Public Market Data structs

type ServerTime struct {
	UnixTime int    `json:"unixtime"`
	Rfc1123  string `json:"rfc1123"`
}

type SystemStatus struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type AssetInfo struct {
	Ticker          string
	Aclass          string  `json:"aclass"`
	Altname         string  `json:"altname"`
	Decimals        uint8   `json:"decimals"`
	DisplayDecimals uint8   `json:"display_decimals"`
	CollateralValue float32 `json:"collateral_value"`
	Status          string  `json:"status"`
}

type AssetPairInfo struct {
	Ticker             string
	Altname            string      `json:"altname"`
	Wsname             string      `json:"wsname"`
	AclassBase         string      `json:"aclass_base"`
	Base               string      `json:"base"`
	AclassQuote        string      `json:"aclass_quote"`
	Quote              string      `json:"quote"`
	CostDecimals       uint8       `json:"cost_decimals"`
	PairDecimals       uint8       `json:"pair_decimals"`
	LotDecimals        uint8       `json:"lot_decimals"`
	LotMultiplier      uint8       `json:"lot_multiplier"`
	LeverageBuy        []uint8     `json:"leverage_buy"`
	LeverageSell       []uint8     `json:"leverage_sell"`
	Fees               [][]float64 `json:"fees"`
	FeesMaker          [][]float64 `json:"fees_maker"`
	FeeVolumeCurrency  string      `json:"fee_volume_currency"`
	MarginCall         uint8       `json:"margin_call"`
	MarginStop         uint8       `json:"margin_stop"`
	OrderMin           string      `json:"ordermin"`
	CostMin            string      `json:"costmin"`
	TickSize           string      `json:"tick_size"`
	Status             string      `json:"status"`
	LongPositionLimit  uint32      `json:"long_position_limit"`
	ShortPositionLimit uint32      `json:"short_position_limit"`
}

type AssetPairMargin struct {
	Ticker      string
	MarginCall  uint8 `json:"margin_call"`
	MarginLevel uint8 `json:"margin_level"`
}

type AssetPairFees struct {
	Ticker            string
	Fees              [][]float64 `json:"fees"`
	FeesMaker         [][]float64 `json:"fees_maker"`
	FeeVolumeCurrency string      `json:"fee_volume_currency"`
}

type AssetPairLeverage struct {
	Ticker       string
	LeverageBuy  []uint8 `json:"leverage_buy"`
	LeverageSell []uint8 `json:"leverage_sell"`
}

type TickerInfo struct {
	Ticker          string
	Ask             TickerBookInfo      `json:"a"`
	Bid             TickerBookInfo      `json:"b"`
	LastTradeClosed TickerLastTradeInfo `json:"c"`
	Volume          TickerDailyInfo     `json:"v"`
	VWAP            TickerDailyInfo     `json:"p"`
	NumberOfTrades  TickerDailyInfoInt  `json:"t"`
	Low             TickerDailyInfo     `json:"l"`
	High            TickerDailyInfo     `json:"h"`
	Open            string              `json:"o"`
}

type TickerBookInfo struct {
	Price          string
	WholeLotVolume string
	LotVolume      string
}

type TickerLastTradeInfo struct {
	Price     string
	LotVolume string
}

type TickerDailyInfo struct {
	Today       string
	Last24Hours string
}

type TickerDailyInfoInt struct {
	Today       int
	Last24Hours int
}

func (ti *TickerBookInfo) UnmarshalJSON(data []byte) error {
	var v []string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 3 {
		ti.Price = v[0]
		ti.WholeLotVolume = v[1]
		ti.LotVolume = v[2]
	}
	return nil
}

func (ti *TickerLastTradeInfo) UnmarshalJSON(data []byte) error {
	var v []string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 2 {
		ti.Price = v[0]
		ti.LotVolume = v[1]
	}
	return nil
}

func (ti *TickerDailyInfo) UnmarshalJSON(data []byte) error {
	var v []string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 2 {
		ti.Today = v[0]
		ti.Last24Hours = v[1]
	}
	return nil
}

func (ti *TickerDailyInfoInt) UnmarshalJSON(data []byte) error {
	var v []int
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 2 {
		ti.Today = v[0]
		ti.Last24Hours = v[1]
	}
	return nil
}

type TickerVolume struct {
	Ticker string
	Volume float64
}

type TickerTrades struct {
	Ticker    string
	NumTrades int
}

type OHLCResp struct {
	Ticker  string
	Data    OHLCDataSlice
	Current OHLCData
	Last    uint64 `json:"last"`
}

type OHLCDataSlice []OHLCData

type OHLCData struct {
	Time   uint64
	Open   string
	High   string
	Low    string
	Close  string
	VWAP   string
	Volume string
	Count  uint32
}

func (ohlc *OHLCResp) UnmarshalJSON(data []byte) error {
	var dataMap map[string]interface{}
	if err := json.Unmarshal(data, &dataMap); err != nil {
		return fmt.Errorf("error unmarshalling data to map | %w", err)
	}
	if last, ok := dataMap["last"].(float64); ok {
		ohlc.Last = uint64(last)
	} else {
		return fmt.Errorf("\"last\" assertion error")
	}
	for key := range dataMap {
		if key != "last" {
			ohlc.Ticker = key
			tempDataSlice, ok := dataMap[key].([]interface{})
			if !ok {
				return fmt.Errorf("OHLCDataSlice assertion error")
			}
			ohlc.Data = make(OHLCDataSlice, len(tempDataSlice)-1)
			for i, v := range tempDataSlice {
				item, ok := v.([]interface{})
				if !ok {
					return fmt.Errorf("OHLCData item assertion error")
				}
				ohlcData := OHLCData{
					Time:   uint64(item[0].(float64)),
					Open:   item[1].(string),
					High:   item[2].(string),
					Low:    item[3].(string),
					Close:  item[4].(string),
					VWAP:   item[5].(string),
					Volume: item[6].(string),
					Count:  uint32(item[7].(float64)),
				}
				if i == len(tempDataSlice)-1 {
					ohlc.Current = ohlcData
				} else {
					ohlc.Data[i] = ohlcData
				}
			}
		}
	}
	return nil
}

type OrderBook struct {
	Ticker string
	Asks   []BookEntry `json:"asks"`
	Bids   []BookEntry `json:"bids"`
}

type BookEntry struct {
	Price  string
	Volume string
	Time   uint64
}

func (be *BookEntry) UnmarshalJSON(data []byte) error {
	var v []interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 3 {
		be.Price = v[0].(string)
		be.Volume = v[1].(string)
		be.Time = uint64(v[2].(float64))
	}
	return nil
}

type TradesResp struct {
	Ticker string
	Trades TradeSlice
	Last   float64 `json:"last"`
}

type TradeSlice []Trade

type Trade struct {
	Price     string
	Volume    string
	Time      float64
	Direction string
	OrderType string
	Misc      string
}

func (tr *TradesResp) UnmarshalJSON(data []byte) error {
	var err error
	dataMap := make(map[string]interface{})
	json.Unmarshal(data, &dataMap)
	lastStr, ok := dataMap["last"].(string)
	if !ok {
		err = fmt.Errorf("error asserting 'last' to string")
		return err
	}
	tr.Last, err = strconv.ParseFloat(lastStr, 64)
	if err != nil {
		return err
	}
	for key, data := range dataMap {
		if key != "last" {
			tr.Ticker = key
			tradeData, ok := data.([]interface{})
			if !ok {
				err = fmt.Errorf("error asserting 'data' to TradeSlice")
				return err
			}
			trades := make(TradeSlice, len(tradeData))
			for i, td := range tradeData {
				tradeInfo, ok := td.([]interface{})
				if !ok || len(tradeInfo) != 6 {
					err = fmt.Errorf("error asserting 'tradeData' to []interface{} or not enough data")
					return err
				}
				trades[i] = Trade{
					Price:     tradeInfo[0].(string),
					Volume:    tradeInfo[1].(string),
					Time:      tradeInfo[2].(float64),
					Direction: tradeInfo[3].(string),
					OrderType: tradeInfo[4].(string),
					Misc:      tradeInfo[5].(string),
				}
			}
			tr.Trades = trades
			break
		}
	}
	return nil
}

type SpreadResp struct {
	Ticker  string
	Spreads SpreadSlice
	Last    uint64 `json:"last"`
}

type SpreadSlice []Spread

type Spread struct {
	Time uint64
	Bid  string
	Ask  string
}

func (sr *SpreadResp) UnmarshalJSON(data []byte) error {
	var err error
	dataMap := make(map[string]interface{})
	json.Unmarshal(data, &dataMap)
	lastStr, ok := dataMap["last"].(float64)
	if !ok {
		err = fmt.Errorf("error asserting 'last' to string")
		return err
	}
	sr.Last = uint64(lastStr)
	for key, data := range dataMap {
		if key != "last" {
			sr.Ticker = key
			spreadData, ok := data.([]interface{})
			if !ok {
				err = fmt.Errorf("error asserting 'data' to []interface{}")
				return err
			}
			spreads := make(SpreadSlice, len(spreadData))
			for i, sd := range spreadData {
				spread, ok := sd.([]interface{})
				if !ok || len(spread) != 3 {
					err = fmt.Errorf("error asserting 'sd' to []interface{} or not enough data")
					return err
				}
				spreads[i] = Spread{
					Time: uint64(spread[0].(float64)),
					Bid:  spread[1].(string),
					Ask:  spread[2].(string),
				}
			}
			sr.Spreads = spreads
			break
		}
	}
	return nil
}

// #endregion

// #region Private Account Data structs

type ExtendedBalance struct {
	Balance    string `json:"balance"`
	Credit     string `json:"credit"`
	CreditUsed string `json:"credit_used"`
	HoldTrade  string `json:"hold_trade"`
}

type TradeBalance struct {
	EquivalentBalance string `json:"eb"`
	TradeBalance      string `json:"tb"`
	OpenMargin        string `json:"m"`
	UnrealizedPnL     string `json:"n"`
	CostBasis         string `json:"c"`
	FloatingValuation string `json:"v"`
	Equity            string `json:"e"`
	FreeMargin        string `json:"mf"`
	MarginLevel       string `json:"ml"`
	UnexecutedValue   string `json:"uv"`
}

type OpenOrdersResp struct {
	OpenOrders map[string]Order `json:"open"`
}

type ClosedOrdersResp struct {
	ClosedOrders map[string]Order `json:"closed"`
	Count        int              `json:"count"`
}

type Order struct {
	RefID          string           `json:"refid"`
	UserRef        int              `json:"userref"`
	Status         string           `json:"status"`
	OpenTime       float64          `json:"opentm"`
	StartTime      float64          `json:"starttm"`
	ExpireTime     float64          `json:"expiretm"`
	Description    OrderDescription `json:"descr"`
	Volume         string           `json:"vol"`
	VolumeExecuted string           `json:"vol_exec"`
	QuoteCost      string           `json:"cost"`
	QuoteFee       string           `json:"fee"`
	AvgPrice       string           `json:"price"`
	StopPrice      string           `json:"stopprice"`
	LimitPrice     string           `json:"limitprice"`
	Trigger        string           `json:"trigger"`
	Misc           string           `json:"misc"`
	OrderFlags     string           `json:"oflags"`
	TradeIDs       []string         `json:"trades"`
	CloseTime      float64          `json:"closetm,omitempty"`
	Reason         string           `json:"reason,omitempty"`
}

type OrderDescription struct {
	Pair             string `json:"pair"`
	Direction        string `json:"type"`
	OrderType        string `json:"ordertype"`
	Price            string `json:"price"`  // Limit price for limit orders. Trigger price for stop-loss, stop-loss-limit, take-profit, take-profit-limit, trailing-stop, and trailing-stop-limit orders
	Price2           string `json:"price2"` // Secondary limit price for stop-loss-limit, take-profit-limit, and trailing-stop-limit orders
	Leverage         string `json:"leverage"`
	Description      string `json:"order"`
	CloseDescription string `json:"close"`
}

type TradesHistoryResp struct {
	Trades map[string]TradeInfo `json:"trades"`
}

type TradeInfo struct {
	OrderTxID           string   `json:"ordertxid"`
	PositionTxID        string   `json:"postxid"`
	Pair                string   `json:"pair"`
	Time                float64  `json:"time"`
	Direction           string   `json:"type"`
	OrderType           string   `json:"ordertype"`
	AvgPrice            string   `json:"price"`
	QuoteCost           string   `json:"cost"`
	QuoteFee            string   `json:"fee"`
	Volume              string   `json:"vol"`
	InitialMargin       string   `json:"margin"`
	Leverage            string   `json:"leverage"`
	Misc                string   `json:"misc"`
	TradeID             int      `json:"trade_id"`
	PositionStatus      string   `json:"posstatus"`
	PortionClosedPrice  string   `json:"cprice"`
	PortionClosedCost   string   `json:"ccost"`
	PortionClosedFee    string   `json:"cfee"`
	PortionClosedVolume string   `json:"cvol"`
	PortionMarginFreed  string   `json:"cmargin"`
	PortionClosedPnL    string   `json:"net"`
	Trades              []string `json:"trades"`
}

type OpenPosition struct {
	OrderTxID      string  `json:"ordertxid"`
	PositionStatus string  `json:"posstatus"`
	Pair           string  `json:"pair"`
	Time           float64 `json:"time"`
	Direction      string  `json:"type"`
	OrderType      string  `json:"ordertype"`
	QuoteCost      string  `json:"cost"`
	QuoteFee       string  `json:"fee"`
	Size           string  `json:"vol"`
	VolumeClosed   string  `json:"vol_closed"`
	InitialMargin  string  `json:"margin"`
	CurrentValue   string  `json:"value"`
	UPnL           string  `json:"net"`
	Terms          string  `json:"terms"`
	RolloverTime   string  `json:"rollovertm"`
	Misc           string  `json:"misc"`
	OrderFlags     string  `json:"oflags"`
}

type OpenPositionConsolidated struct {
	Pair          string `json:"pair"`
	Direction     string `json:"type"`
	Size          string `json:"vol"`
	VolumeClosed  string `json:"vol_closed"`
	QuoteCost     string `json:"cost"`
	QuoteFee      string `json:"fee"`
	Leverage      string `json:"leverage"`
	InitialMargin string `json:"margin"`
	UPnL          string `json:"net"`
	NumPositions  string `json:"positions"`
	CurrentValue  string `json:"value"`
}

type LedgersInfoResp struct {
	Ledgers map[string]Ledger `json:"ledger"`
	Count   int               `json:"count"`
}

type Ledger struct {
	RefID      string  `json:"refid"`
	Time       float64 `json:"time"`
	Type       string  `json:"type"`
	SubType    string  `json:"subtype"`
	AssetClass string  `json:"aclass"`
	Asset      string  `json:"asset"`
	TxAmount   string  `json:"amount"`
	TxFee      string  `json:"fee"`
	EndBalance string  `json:"balance"`
}

type TradeVolume struct {
	Currency      string         `json:"currency"`
	CurrentVolume string         `json:"volume"`
	Fees          map[string]Fee `json:"fees"`
	MakerFees     map[string]Fee `json:"fees_maker"`
}

type Fee struct {
	Fee        string `json:"fee"`
	MinFee     string `json:"min_fee"`
	MaxFee     string `json:"max_fee"`
	NextFee    string `json:"next_fee"`
	TierVolume string `json:"tier_volume"`
	NextVolume string `json:"next_volume"`
}

type RequestExportReportResp struct {
	ID string `json:"id"`
}

type ExportReportStatus struct {
	ID            string `json:"id"`
	Description   string `json:"descr"`
	Format        string `json:"format"`
	Report        string `json:"report"`
	SubType       string `json:"subtype"`
	Status        string `json:"status"`
	Fields        string `json:"fields"`
	CreatedTime   string `json:"createdtm"`
	StartTime     string `json:"starttm"`
	CompletedTime string `json:"completedtm"`
	DataStartTime string `json:"datastarttm"`
	DataEndTime   string `json:"dataendtm"`
	Asset         string `json:"asset"`
}

type DeleteReportResp struct {
	Delete bool `json:"delete"`
	Cancel bool `json:"cancel"`
}

// #endregion

// #region Private Trading Data structs

type AddOrderResp struct {
	Description AddOrderDescription `json:"descr"`
	TxID        []string            `json:"txid"`
}

type AddOrderDescription struct {
	OrderDescription string `json:"order"`
	CloseDescription string `json:"close"`
}

type AddOrderBatchResp struct {
	Orders []BatchResp `json:"orders"`
}

type BatchResp struct {
	Description BatchOrderDescription `json:"descr"`
	Error       string                `json:"error"`
	TxID        string                `json:"txid"`
}

type BatchOrderDescription struct {
	OrderDescription string `json:"order"`
}

type EditOrderResp struct {
	Description        EditOrderDescription `json:"descr"`
	NewTxID            string               `json:"txid"`
	Volume             string               `json:"volume"`
	Price              string               `json:"price"`
	Price2             string               `json:"price2"`
	NewUserRef         int                  `json:"newuserref"`
	OldUserRef         int                  `json:"olduserref"`
	NumOrdersCancelled uint8                `json:"orders_cancelled"`
	OldTxID            string               `json:"originaltxid"`
	Status             string               `json:"status"`
	ErrorMessage       string               `json:"error_message"`
}

type EditOrderDescription struct {
	OrderDescription string `json:"order"`
}

type CancelOrderResp struct {
	Count   int  `json:"count"`
	Pending bool `json:"pending"`
}

type CancelAllAfter struct {
	CurrentTime string `json:"currentTime"`
	TriggerTime string `json:"triggerTime"`
}

// #endregion

// #region Private Funding Data structs

type DepositMethod struct {
	Method             string      `json:"method"`
	MinDeposit         string      `json:"minimum"`
	MaxDeposit         interface{} `json:"limit"`
	Fee                string      `json:"fee"`
	AddressSetupFee    string      `json:"address-setup-fee"`
	CanGenerateAddress bool        `json:"gen-address"`
}

type DepositAddress struct {
	Address    string      `json:"address"`
	ExpireTime string      `json:"expiretm"`
	New        bool        `json:"new"`
	Memo       string      `json:"memo"`
	Tag        interface{} `json:"tag"`
}

type DepositStatus struct {
	Method         string      `json:"method"`
	AssetClass     string      `json:"aclass"`
	Asset          string      `json:"asset"`
	RefID          string      `json:"refid"`
	TxID           string      `json:"txid"`
	Info           string      `json:"info"`
	Amount         string      `json:"amount"`
	Fee            interface{} `json:"fee"`
	TimeRequested  int32       `json:"time"`
	Status         interface{} `json:"status"`
	StatusProperty string      `json:"status-prop"`
	Originators    []string    `json:"originators"`
}

type DepositStatusPaginated struct {
	Deposits   []DepositStatus `json:"deposits"`
	NextCursor string          `json:"next_cursor"`
}

type WithdrawalMethod struct {
	Asset   string `json:"asset"`
	Method  string `json:"method"`
	Network string `json:"network"`
	Minimum string `json:"minimum"`
}

type WithdrawalAddress struct {
	Address  string `json:"address"`
	Asset    string `json:"asset"`
	Method   string `json:"method"`
	Key      string `json:"key"`
	Memo     string `json:"memo"`
	Verified bool   `json:"verified"`
}

type WithdrawalInfo struct {
	Method string `json:"method"`
	Limit  string `json:"limit"`
	Amount string `json:"amount"`
	Fee    string `json:"fee"`
}

type WithdrawFundsResponse struct {
	RefID string `json:"refid"`
}

type WithdrawalStatus struct {
	Method         string      `json:"method"`
	Network        string      `json:"network"`
	AssetClass     string      `json:"aclass"`
	Asset          string      `json:"asset"`
	RefID          string      `json:"refid"`
	TxID           string      `json:"txid"`
	Info           string      `json:"info"`
	Amount         string      `json:"amount"`
	Fee            interface{} `json:"fee"`
	TimeRequested  int32       `json:"time"`
	Status         string      `json:"status"`
	StatusProperty string      `json:"status-prop"`
	Key            string      `json:"key"`
}

type WithdrawalStatusPaginated struct {
	Withdrawals []WithdrawalStatus `json:"withdrawals"`
	NextCursor  string             `json:"next_cursor"`
}

type WalletTransferResponse struct {
	RefID string `json:"refid"`
}

// #endregion

// #region Private Subaccounts Data structs

type AccountTransfer struct {
	TransferID string `json:"transfer_id"`
	Status     string `json:"status"`
}

// #endregion

// #region Private Earn Data structs

type AllocationStatus struct {
	Pending bool `json:"pending"`
}

type EarnStrategiesResp struct {
	Strategies []EarnStrategy `json:"items"`
	NextCursor string         `json:"next_cursor"`
}

type EarnStrategy struct {
	Asset             string       `json:"asset"`
	AllocationFee     interface{}  `json:"allocation_fee"`
	RestrictionInfo   []string     `json:"allocation_restriction_info"`
	APREstimate       APREstimate  `json:"apr_estimate"`
	AutoCompound      AutoCompound `json:"auto_compound"`
	CanAllocate       bool         `json:"can_allocate"`
	CanDeallocate     bool         `json:"can_deallocate"`
	DeallocationFee   interface{}  `json:"deallocation_fee"`
	ID                string       `json:"id"`
	LockType          LockType     `json:"lock_type"`
	UserCap           string       `json:"user_cap"`
	UserMinAllocation string       `json:"user_min_allocation"`
	YieldSource       YieldSource  `json:"yield_source"`
}

type APREstimate struct {
	High string `json:"high"`
	Low  string `json:"low"`
}

type AutoCompound struct {
	Default bool   `json:"default"`
	Type    string `json:"type"`
}

type LockType struct {
	Type                    string `json:"type"`
	PayoutFrequency         int    `json:"payout_frequency"`
	BondingPeriod           int    `json:"bonding_period"`
	BondingPeriodVariable   bool   `json:"bonding_period_variable"`
	BondingRewards          bool   `json:"bonding_rewards"`
	ExitQueuePeriod         int    `json:"exit_queue_period"`
	UnbondingPeriod         int    `json:"unbonding_period"`
	UnbondingPeriodVariable bool   `json:"unbonding_period_variable"`
	UnbondingRewards        bool   `json:"unbonding_rewards"`
}

type YieldSource struct {
	Type string `json:"type"`
}

type EarnAllocationsResp struct {
	ConvertedAsset string           `json:"converted_asset"`
	Allocations    []EarnAllocation `json:"items"`
	TotalAllocated string           `json:"total_allocated"`
	TotalRewarded  string           `json:"total_rewarded"`
}

type EarnAllocation struct {
	AmountAllocated EarnAlloAmount `json:"amount_allocated"`
	NativeAsset     string         `json:"native_asset"`
	Payout          EarnAlloPayout `json:"payout"`
	StrategyID      string         `json:"strategy_id"`
	TotalReward     EarnAlloReward `json:"total_rewarded"`
}

type EarnAlloAmount struct {
	Bonding   AmountAllocated `json:"bonding"`
	ExitQueue AmountAllocated `json:"exit_queue"`
	Pending   AmountAllocated `json:"pending"`
	Total     AmountAllocated `json:"total"`
	Unbonding AmountAllocated `json:"unbonding"`
}

type AmountAllocated struct {
	AllocationCount uint         `json:"allocation_count"`
	Allocations     []Allocation `json:"allocations"`
	ConvertedAmount string       `json:"converted"`
	NativeAmount    string       `json:"native"`
}

type Allocation struct {
	Converted    string `json:"converted"`
	TimeCreated  string `json:"created_at"`
	TimeExpires  string `json:"expires"`
	NativeAmount string `json:"native"`
}

type EarnAlloPayout struct {
	AccumulatedReward EarnAlloReward `json:"accumulated_reward"`
	EstimatedReward   EarnAlloReward `json:"estimated_reward"`
	PeriodEnd         string         `json:"period_end"`
	PeriodStart       string         `json:"period_start"`
}

type EarnAlloReward struct {
	ConvertedAmount string `json:"converted"`
	NativeAmount    string `json:"native"`
}

// #endregion

// #region Private WebSockets Authentication Data structs

type WebSocketsToken struct {
	Token   string `json:"token"`
	Expires uint16 `json:"expires"`
}

// #endregion

// #region Public WebSockets data structs

type GenericMessage struct {
	Event   string `json:"event"`
	Content interface{}
}

func (gm *GenericMessage) UnmarshalJSON(data []byte) error {
	type Alias GenericMessage
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(gm),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	switch gm.Event {
	case "subscriptionStatus":
		var msg WSSubscriptionStatus
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("error unmarshalling subscriptionstatus msg | %w", err)
		}
		gm.Content = msg
	case "systemStatus":
		var msg WSSystemStatus
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("error unmarshalling systemstatus msg | %w", err)
		}
		gm.Content = msg
	case "pong":
		var msg WSPong
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("error unmarshalling pong msg | %w", err)
		}
		gm.Content = msg
	default:
		return fmt.Errorf("unknown event type | %s", gm.Event)
	}
	return nil
}

type GenericArrayMessage struct {
	ChannelName string
	Content     interface{}
}

func (gm *GenericArrayMessage) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("error unmarshalling json | %w", err)
	}
	if err := json.Unmarshal(raw[len(raw)-2], &gm.ChannelName); err != nil {
		return fmt.Errorf("error unmarshalling json | %w", err)
	}
	switch {
	case gm.ChannelName == "ticker":
		var content WSTickerResp
		if err := json.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("error unmarshalling json to wstickerresp type | %w", err)
		}
		gm.Content = content
	case strings.HasPrefix(gm.ChannelName, "ohlc"):
		var content WSOHLCResp
		if err := json.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("error unmarshalling json to wsohlcresp type | %w", err)
		}
		gm.Content = content
	case gm.ChannelName == "trade":
		var content WSTradeResp
		if err := json.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("error unmarshalling json to wstraderesp type | %w", err)
		}
		gm.Content = content
	case gm.ChannelName == "spread":
		var content WSSpreadResp
		if err := json.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("error unmarshalling json to wsspreadresp type | %w", err)
		}
		gm.Content = content
	case strings.HasPrefix(gm.ChannelName, "book"):
		var content WSBookResp
		if err := json.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("error unmarshalling json to wsbookresp type | %w", err)
		}
		gm.Content = content
	default:
		return fmt.Errorf("cannot unmarshal unknown channel name | %s", gm.ChannelName)
	}
	return nil
}

type WSPong struct {
	Event string `json:"event"`
	ReqID int    `json:"reqid"`
}

type WSSystemStatus struct {
	ConnectionID uint64 `json:"connectionID"`
	Event        string `json:"event"`
	Status       string `json:"status"`
	Version      string `json:"version"`
}

type WSSubscriptionStatus struct {
	ChannelID    int            `json:"channelID"`
	ErrorMessage string         `json:"errorMessage"`
	ChannelName  string         `json:"channelName"`
	Event        string         `json:"event"`
	RequestID    int            `json:"reqid"`
	Pair         string         `json:"pair"`
	Status       string         `json:"status"`
	Subscription WSSubscription `json:"subscription"`
}

type WSSubscription struct {
	Depth        int    `json:"depth"`
	Interval     int    `json:"interval"`
	MaxRateCount int    `json:"maxratecount"`
	Name         string `json:"name"`
	Token        string `json:"token"`
}

type WSTickerResp struct {
	ChannelID   int `json:"channelID"`
	TickerInfo  WSTickerInfo
	ChannelName string `json:"channelName"`
	Pair        string `json:"pair"`
}

func (w *WSTickerResp) UnmarshalJSON(data []byte) error {
	var rawMessage []json.RawMessage
	if err := json.Unmarshal(data, &rawMessage); err != nil {
		return err
	}
	if len(rawMessage) != 4 {
		err := fmt.Errorf("unexpected JSON array length")
		return err
	}
	if err := json.Unmarshal(rawMessage[0], &w.ChannelID); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[1], &w.TickerInfo); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[2], &w.ChannelName); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[3], &w.Pair); err != nil {
		return err
	}
	return nil
}

type WSTickerInfo struct {
	Ticker          string
	Ask             WSTickerBook        `json:"a"`
	Bid             WSTickerBook        `json:"b"`
	LastTradeClosed WSTickerLastTrade   `json:"c"`
	Volume          WSTickerDaily       `json:"v"`
	VWAP            WSTickerDaily       `json:"p"`
	NumberOfTrades  WSTickerDailyTrades `json:"t"`
	Low             WSTickerDaily       `json:"l"`
	High            WSTickerDaily       `json:"h"`
	Open            WSTickerDaily       `json:"o"`
}

func (w *WSTickerBook) UnmarshalJSON(data []byte) error {
	var rawMessage []json.RawMessage
	if err := json.Unmarshal(data, &rawMessage); err != nil {
		return err
	}
	if len(rawMessage) != 3 {
		err := fmt.Errorf("unexpected JSON array length")
		return err
	}
	if err := json.Unmarshal(rawMessage[0], &w.Price); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[1], &w.WholeLotVolume); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[2], &w.LotVolume); err != nil {
		return err
	}
	return nil
}

type WSTickerBook struct {
	Price          string `json:"price"`
	WholeLotVolume uint64 `json:"wholeLotVolume"`
	LotVolume      string `json:"lotVolume"`
}

func (w *WSTickerLastTrade) UnmarshalJSON(data []byte) error {
	var rawMessage []json.RawMessage
	if err := json.Unmarshal(data, &rawMessage); err != nil {
		return err
	}
	if len(rawMessage) != 2 {
		err := fmt.Errorf("unexpected JSON array length")
		return err
	}
	if err := json.Unmarshal(rawMessage[0], &w.Price); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[1], &w.LotVolume); err != nil {
		return err
	}
	return nil
}

type WSTickerLastTrade struct {
	Price     string `json:"price"`
	LotVolume string `json:"lotVolume"`
}

func (w *WSTickerDaily) UnmarshalJSON(data []byte) error {
	var rawMessage []json.RawMessage
	if err := json.Unmarshal(data, &rawMessage); err != nil {
		return err
	}
	if len(rawMessage) != 2 {
		err := fmt.Errorf("unexpected JSON array length")
		return err
	}
	if err := json.Unmarshal(rawMessage[0], &w.Today); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[1], &w.Last24Hours); err != nil {
		return err
	}
	return nil
}

type WSTickerDaily struct {
	Today       string `json:"today"`
	Last24Hours string `json:"last24Hours"`
}

func (w *WSTickerDailyTrades) UnmarshalJSON(data []byte) error {
	var rawMessage []json.RawMessage
	if err := json.Unmarshal(data, &rawMessage); err != nil {
		return err
	}
	if len(rawMessage) != 2 {
		err := fmt.Errorf("unexpected JSON array length")
		return err
	}
	if err := json.Unmarshal(rawMessage[0], &w.Today); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[1], &w.Last24Hours); err != nil {
		return err
	}
	return nil
}

type WSTickerDailyTrades struct {
	Today       uint `json:"today"`
	Last24Hours uint `json:"last24Hours"`
}

type WSOHLCResp struct {
	ChannelID   int `json:"channelID"`
	OHLC        WSOHLC
	ChannelName string `json:"channelName"`
	Pair        string `json:"pair"`
}

func (w *WSOHLCResp) UnmarshalJSON(data []byte) error {
	var rawMessage []json.RawMessage
	if err := json.Unmarshal(data, &rawMessage); err != nil {
		return err
	}
	if len(rawMessage) != 4 {
		err := fmt.Errorf("unexpected JSON array length")
		return err
	}
	if err := json.Unmarshal(rawMessage[0], &w.ChannelID); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[1], &w.OHLC); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[2], &w.ChannelName); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[3], &w.Pair); err != nil {
		return err
	}
	return nil
}

type WSOHLC struct {
	Time    string
	EndTime string
	Open    string
	High    string
	Low     string
	Close   string
	VWAP    string
	Volume  string
	Count   int
}

func (w *WSOHLC) UnmarshalJSON(data []byte) error {
	var rawMessage []json.RawMessage
	if err := json.Unmarshal(data, &rawMessage); err != nil {
		return err
	}
	if len(rawMessage) != 9 {
		err := fmt.Errorf("unexpected JSON array length")
		return err
	}
	if err := json.Unmarshal(rawMessage[0], &w.Time); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[1], &w.EndTime); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[2], &w.Open); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[3], &w.High); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[4], &w.Low); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[5], &w.Close); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[6], &w.VWAP); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[7], &w.Volume); err != nil {
		return err
	}
	if err := json.Unmarshal(rawMessage[8], &w.Count); err != nil {
		return err
	}
	return nil
}

type WSTradeResp struct {
	ChannelID   int `json:"channelID"`
	Trades      []WSTrade
	ChannelName string `json:"channelName"`
	Pair        string `json:"pair"`
}

func (t *WSTradeResp) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("error unmarshalling json | %w", err)
	}
	if len(raw) != 4 {
		return fmt.Errorf("encountered unexpected data length during unmarshal")
	}
	if err := json.Unmarshal(raw[0], &t.ChannelID); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &t.Trades); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[2], &t.ChannelName); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[3], &t.Pair); err != nil {
		return err
	}
	return nil
}

type WSTrade struct {
	Price     string
	Volume    string
	Time      string
	Direction string
	OrderType string
	Misc      string
}

func (t *WSTrade) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return fmt.Errorf("error umarshalling to raw")
	}
	if len(raw) != 6 {
		return fmt.Errorf("unexpected data length")
	}
	if err := json.Unmarshal(raw[0], &t.Price); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &t.Volume); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[2], &t.Time); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[3], &t.Direction); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[4], &t.OrderType); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[5], &t.Misc); err != nil {
		return err
	}
	return nil
}

type WSSpreadResp struct {
	ChannelID   int `json:"channelID"`
	Spread      WSSpread
	ChannelName string `json:"channelName"`
	Pair        string `json:"pair"`
}

func (s *WSSpreadResp) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("error unmarshalling to raw | %w", err)
	}
	if len(raw) != 4 {
		return fmt.Errorf("unexpected data length encountered")
	}
	if err := json.Unmarshal(raw[0], &s.ChannelID); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &s.Spread); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[2], &s.ChannelName); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[3], &s.Pair); err != nil {
		return err
	}
	return nil
}

type WSSpread struct {
	Bid       string
	Ask       string
	Time      string
	BidVolume string
	AskVolume string
}

func (s *WSSpread) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("error unmarshalling to raw | %w", err)
	}
	if len(raw) != 5 {
		return fmt.Errorf("unexpected data length encountered")
	}
	if err := json.Unmarshal(raw[0], &s.Bid); err != nil {
		return fmt.Errorf("error unmarshalling json | %w", err)
	}
	if err := json.Unmarshal(raw[1], &s.Ask); err != nil {
		return fmt.Errorf("error unmarshalling json | %w", err)
	}
	if err := json.Unmarshal(raw[2], &s.Time); err != nil {
		return fmt.Errorf("error unmarshalling json | %w", err)
	}
	if err := json.Unmarshal(raw[3], &s.BidVolume); err != nil {
		return fmt.Errorf("error unmarshalling json | %w", err)
	}
	if err := json.Unmarshal(raw[4], &s.AskVolume); err != nil {
		return fmt.Errorf("error unmarshalling json | %w", err)
	}
	return nil
}

type WSBookResp struct {
	ChannelID   int `json:"channelID"`
	OrderBook   WSOrderBook
	ChannelName string `json:"channelName"`
	Pair        string `json:"pair"`
}

func (s *WSBookResp) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("error unmarshalling to raw | %w", err)
	}
	if len(raw) != 4 {
		return fmt.Errorf("unexpected data length encountered")
	}
	if err := json.Unmarshal(raw[0], &s.ChannelID); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[1], &s.OrderBook); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[2], &s.ChannelName); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[3], &s.Pair); err != nil {
		return err
	}
	return nil
}

type WSOrderBook struct {
	Asks     []WSBookEntry
	Bids     []WSBookEntry
	Checksum string `json:"c"`
}

func (ob *WSOrderBook) UnmarshalJSON(data []byte) error {
	type Alias WSOrderBook
	aux := &struct {
		AsksA  []WSBookEntry `json:"a"`
		AsksAS []WSBookEntry `json:"as"`
		BidsB  []WSBookEntry `json:"b"`
		BidsBS []WSBookEntry `json:"bs"`
		*Alias
	}{
		Alias: (*Alias)(ob),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.AsksA != nil {
		ob.Asks = aux.AsksA
	} else if aux.AsksAS != nil {
		ob.Asks = aux.AsksAS
	}
	if aux.BidsB != nil {
		ob.Bids = aux.BidsB
	} else if aux.BidsBS != nil {
		ob.Bids = aux.BidsBS
	}
	return nil
}

type WSBookEntry struct {
	Price      string
	Volume     string
	Time       string
	UpdateType string
}

func (s *WSBookEntry) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("error unmarshalling to raw | %w", err)
	}
	switch len(raw) {
	case 3:
		if err := json.Unmarshal(raw[0], &s.Price); err != nil {
			return fmt.Errorf("error unmarshalling json | %w", err)
		}
		if err := json.Unmarshal(raw[1], &s.Volume); err != nil {
			return fmt.Errorf("error unmarshalling json | %w", err)
		}
		if err := json.Unmarshal(raw[2], &s.Time); err != nil {
			return fmt.Errorf("error unmarshalling json | %w", err)
		}
	case 4:
		if err := json.Unmarshal(raw[0], &s.Price); err != nil {
			return fmt.Errorf("error unmarshalling json | %w", err)
		}
		if err := json.Unmarshal(raw[1], &s.Volume); err != nil {
			return fmt.Errorf("error unmarshalling json | %w", err)
		}
		if err := json.Unmarshal(raw[2], &s.Time); err != nil {
			return fmt.Errorf("error unmarshalling json | %w", err)
		}
		if err := json.Unmarshal(raw[3], &s.UpdateType); err != nil {
			return fmt.Errorf("error unmarshalling json | %w", err)
		}
	default:
		return fmt.Errorf("unexpected data length encountered")
	}
	return nil
}

// #endregion

// #region InternalOrderBook data structs

type InternalOrderBook struct {
	Asks           []InternalBookEntry
	Bids           []InternalBookEntry
	DataChan       chan WSOrderBook
	DoneChan       chan struct{}
	DataChanClosed int32
	DoneChanClosed int32
	Mutex          sync.RWMutex
}

type InternalBookEntry struct {
	Price  decimal.Decimal
	Volume decimal.Decimal
	Time   decimal.Decimal
}

type BookState struct {
	Asks *[]InternalBookEntry
	Bids *[]InternalBookEntry
}

// #endregion
