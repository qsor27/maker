// Copyright (C) 2018 Cranky Kernel
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package binanceex

import (
	"gitlab.com/crankykernel/cryptotrader/binance"
	"gitlab.com/crankykernel/maker/log"
	"strings"
	"sync"
	"time"
)

type BinanceStreamManager struct {
	mutex                          sync.Mutex
	tradeStreamCount               map[string]uint
	tradeStreamUnsubscribeChannels map[string]chan bool
	tradeStreamSubscriptions       map[chan *binance.StreamAggTrade]bool
}

func NewBinanceStreamManager() *BinanceStreamManager {
	return &BinanceStreamManager{
		tradeStreamCount:               make(map[string]uint),
		tradeStreamSubscriptions:       make(map[chan *binance.StreamAggTrade]bool),
		tradeStreamUnsubscribeChannels: make(map[string]chan bool),
	}
}

func (m *BinanceStreamManager) lock() {
	m.mutex.Lock()
}

func (m *BinanceStreamManager) unlock() {
	m.mutex.Unlock()
}

func (m *BinanceStreamManager) SubscribeTrades() chan *binance.StreamAggTrade {
	m.lock()
	channel := make(chan *binance.StreamAggTrade)
	m.tradeStreamSubscriptions[channel] = true
	m.unlock()
	return channel
}

func (m *BinanceStreamManager) UnsubscribeTrades(channel chan *binance.StreamAggTrade) {
	m.lock()
	m.tradeStreamSubscriptions[channel] = false
	delete(m.tradeStreamSubscriptions, channel)
	m.unlock()
}

func (m *BinanceStreamManager) SubscribeTradeStream(symbol string) {
	symbol = strings.ToLower(symbol)
	m.lock()
	count, exists := m.tradeStreamCount[symbol]
	if exists && count > 0 {
		log.WithFields(log.Fields{
			"symbol": symbol,
			"count":  count,
		}).Infof("Trade stream already exists.")
		m.tradeStreamCount[symbol] += 1
		m.unlock()
		return
	}
	m.tradeStreamCount[symbol] = 1
	unsubscribeChannel := make(chan bool)
	m.tradeStreamUnsubscribeChannels[symbol] = unsubscribeChannel
	m.unlock()
	go m.RunTradeStream(symbol, unsubscribeChannel)
}

func (m *BinanceStreamManager) UnsubscribeTradeStream(symbol string) {
	symbol = strings.ToLower(symbol)
	m.lock()
	count, exists := m.tradeStreamCount[symbol]
	if !exists {
		m.unlock()
		return
	}
	m.unlock()
	m.tradeStreamCount[symbol] = count - 1
	if m.tradeStreamCount[symbol] == 0 {
		m.tradeStreamUnsubscribeChannels[symbol] <- true
	}
}

func (m *BinanceStreamManager) RunTradeStream(symbol string, unsubscribe chan bool) {
Reopen:
	for {
		log.WithFields(log.Fields{
			"symbol": symbol,
		}).Info("Opening Binance trade stream.")
		streamClient, err := binance.OpenAggTradeStream(symbol)
		if err != nil {
			log.Printf("failed to open aggTrade stream: %v", err)
			return
		}

		// TODO: As we're already running in a go routing, I think we could just
		//     call Next() on the stream client.
		channel := make(chan binance.AggTradeStreamEvent)
		go streamClient.Subscribe(channel)

		for {
			select {
			case <-unsubscribe:
				log.WithFields(log.Fields{
					"symbol": symbol,
				}).Info("Closing Binance trade stream.")
				streamClient.Close()
				return
			case event := <-channel:
				if event.Err != nil {
					log.WithFields(log.Fields{
						"symbol": symbol,
					}).WithError(event.Err).Errorf("Failed to read from trade stream.")
					time.Sleep(100 * time.Millisecond)
					continue Reopen
				}
				m.lock()
				for channel := range m.tradeStreamSubscriptions {
					channel <- event.Trade
				}
				m.unlock()
			}
		}
	}
}
