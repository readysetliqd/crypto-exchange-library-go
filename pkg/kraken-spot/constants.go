package krakenspot

const baseUrl = "https://api.kraken.com"
const publicPrefix = "/0/public/"
const privatePrefix = "/0/private/"

const (
	pairsMapSize   = 745 // As of 12/29/2023 there were 677 tradeable pairs. 10% added for buffer
	assetsMapSize  = 321 // As of 12/29/2023 there were 292 listed assets. 10% added for buffer
	tickersMapSize = 813 // As of 12/29/2023 there were 739 listed tickers. 10% added for buffer
)

const (
	tier1DecayRate uint8 = 3 // seconds per 1 counter decay
	tier2DecayRate uint8 = 2 // seconds per 1 counter decay
	tier3DecayRate uint8 = 1 // seconds per 1 counter decay
)

var decayRateMap = map[uint8]uint8{
	1: tier1DecayRate,
	2: tier2DecayRate,
	3: tier3DecayRate,
}

// Formatting for number of decimals for USD (ZUSD) asset on Kraken
const usdDecimalsFormat = "%.4f"
