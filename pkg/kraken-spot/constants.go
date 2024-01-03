package krakenspot

const baseUrl = "https://api.kraken.com"
const publicPrefix = "/0/public/"
const privatePrefix = "/0/private/"

// As of 12/29/2023 there were 677 tradeable pairs. 10% added for buffer
const pairsMapSize = 745

// As of 12/29/2023 there were 292 listed assets. 10% added for buffer
const assetsMapSize = 321

// As of 12/29/2023 there were 739 listed tickers. 10% added for buffer
const tickersMapSize = 813

// Formatting for number of decimals for USD (ZUSD) asset on Kraken
const usdDecimalsFormat = "%.4f"
