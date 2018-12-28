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

package server

import (
	"encoding/json"
	"fmt"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"gitlab.com/crankykernel/cryptotrader/binance"
	"gitlab.com/crankykernel/maker/binanceex"
	"gitlab.com/crankykernel/maker/db"
	"gitlab.com/crankykernel/maker/log"
	"gitlab.com/crankykernel/maker/tradeservice"
	"gitlab.com/crankykernel/maker/types"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

func archiveTradeHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		tradeId := vars["tradeId"]
		if tradeId == "" {
			WriteJsonError(w, http.StatusBadRequest, "tradeId required")
			return
		}

		logFields := log.Fields{
			"tradeId": tradeId,
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			log.WithFields(logFields).
				Warn("Failed to archive trade, tradeId not found.")
			WriteJsonError(w, http.StatusNotFound, "trade not found")
			return
		}

		if err := tradeService.ArchiveTrade(trade); err != nil {
			log.WithFields(logFields).
				WithError(err).Error("Failed to archive trade.")
			WriteJsonError(w, http.StatusInternalServerError, err.Error())
			return
		}

		log.WithFields(logFields).Info("Trade archived.")
	}
}

func abandonTradeHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		tradeId := vars["tradeId"]
		if tradeId == "" {
			WriteJsonError(w, http.StatusBadRequest, "tradeId required")
			return
		}

		logFields := log.Fields{
			"tradeId": tradeId,
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			log.WithFields(logFields).
				Warn("Failed to abandon trade, tradeId not found.")
			WriteJsonError(w, http.StatusNotFound, "trade not found")
			return
		}

		tradeService.AbandonTrade(trade)
		log.WithFields(logFields).Info("Trade abandoned.")
	}
}

func updateTradeStopLossSettingsHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		if err = r.ParseForm(); err != nil {
			WriteBadRequestError(w)
			return
		}

		var tradeId string
		var enable bool
		var percent float64

		vars := mux.Vars(r)
		tradeId = vars["tradeId"]
		if tradeId == "" {
			WriteBadRequestError(w)
			return
		}

		if enable, err = strconv.ParseBool(r.FormValue("enable")); err != nil {
			WriteBadRequestError(w)
			return
		}
		if percent, err = strconv.ParseFloat(r.FormValue("percent"), 64); err != nil {
			WriteBadRequestError(w)
			return
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			log.Printf("Failed to find trade with ID %s.", tradeId)
			WriteJsonError(w, http.StatusNotFound, "")
		}

		log.Printf("Updating stop loss for trade %s: enable=%v; percent=%v",
			tradeId, enable, percent)
		tradeService.UpdateStopLoss(trade, enable, percent)
	}
}

func updateTradeTrailingProfitSettingsHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		if err = r.ParseForm(); err != nil {
			WriteBadRequestError(w)
			return
		}

		var tradeId string
		var enable bool
		var percent float64
		var deviation float64

		vars := mux.Vars(r)
		tradeId = vars["tradeId"]
		if tradeId == "" {
			WriteBadRequestError(w)
			return
		}

		if enable, err = strconv.ParseBool(r.FormValue("enable")); err != nil {
			WriteBadRequestError(w)
			return
		}
		if percent, err = strconv.ParseFloat(r.FormValue("percent"), 64); err != nil {
			WriteBadRequestError(w)
			return
		}
		if deviation, err = strconv.ParseFloat(r.FormValue("deviation"), 64); err != nil {
			WriteBadRequestError(w)
			return
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			log.Printf("Failed to find trade with ID %s.", tradeId)
			WriteJsonError(w, http.StatusNotFound, "")
		}

		tradeService.UpdateTrailingProfit(trade, enable, percent, deviation)
	}
}

func deleteBuyHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			WriteBadRequestError(w)
			return
		}

		tradeId := r.FormValue("trade_id")
		if tradeId == "" {
			WriteBadRequestError(w)
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			log.WithFields(log.Fields{
				"tradeId": tradeId,
			}).Warnf("Failed to cancel buy order. Trade ID not found.")
			WriteBadRequestError(w)
			return
		}

		log.WithFields(log.Fields{
			"symbol":  trade.State.Symbol,
			"tradeId": tradeId,
		}).Infof("Cancelling buy order.")

		if err := tradeService.CancelBuy(trade); err != nil {
			WriteJsonError(w, http.StatusBadRequest,
				fmt.Sprintf("Failed to cancel buy order: %v", err))
		} else {
			WriteJsonResponse(w, http.StatusOK, nil)
		}
	}
}

func DeleteSellHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		tradeId := r.FormValue("trade_id")

		if tradeId == "" {
			WriteBadRequestError(w)
			return
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			log.WithFields(log.Fields{
				"tradeId": tradeId,
			}).Warnf("Failed to cancel sell order. No trade found for ID.")
			WriteBadRequestError(w)
			return
		}

		log.WithFields(log.Fields{
			"symbol":      trade.State.Symbol,
			"tradeId":     trade.State.TradeID,
			"sellOrderId": trade.State.SellOrderId,
		}).Infof("Cancelling sell order.")

		switch trade.State.Status {
		case types.TradeStatusNew:
			fallthrough
		case types.TradeStatusPendingBuy:
			trade.State.LimitSell.Enabled = false
			db.DbUpdateTrade(trade)
			tradeService.BroadcastTradeUpdate(trade)
			return
		}

		if err := tradeService.CancelSell(trade); err != nil {
			WriteJsonError(w, http.StatusBadRequest,
				fmt.Sprintf("Failed to cancel sell order: %s", string(err.Error())))
		} else {
			WriteJsonResponse(w, http.StatusOK, nil)
		}
	}
}

func limitSellByPercentHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if err := r.ParseForm(); err != nil {
			WriteBadRequestError(w)
			return
		}

		tradeId := vars["tradeId"]
		if tradeId == "" {
			WriteBadRequestError(w)
			return
		}

		percent, err := strconv.ParseFloat(r.FormValue("percent"), 64)
		if err != nil {
			WriteBadRequestError(w)
			return
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			WriteJsonError(w, http.StatusNotFound, "")
			return
		}

		switch trade.State.Status {
		case types.TradeStatusNew:
			fallthrough
		case types.TradeStatusPendingBuy:
			trade.SetLimitSellByPercent(percent)
			db.DbUpdateTrade(trade)
			tradeService.BroadcastTradeUpdate(trade)
			log.WithFields(log.Fields{
				"symbol":  trade.State.Symbol,
				"tradeId": trade.State.TradeID,
				"percent": percent,
			}).Info("Updated limit sell on buy.")
			return
		}

		startTime := time.Now()

		if trade.State.Status == types.TradeStatusPendingSell {
			log.Printf("Cancelling existing sell order.")
			tradeService.CancelSell(trade)
		}

		err = tradeService.LimitSellByPercent(trade, percent)
		if err != nil {
			log.WithError(err).Error("Limit sell order failed.")
			WriteJsonResponse(w, http.StatusBadRequest, err.Error())
		}

		duration := time.Since(startTime)
		log.WithFields(log.Fields{
			"duration": duration,
			"symbol":   trade.State.Symbol,
		}).Debug("Sell order posted.")
	}
}

func limitSellByPriceHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if err := r.ParseForm(); err != nil {
			WriteBadRequestError(w)
			return
		}

		tradeId := vars["tradeId"]
		if tradeId == "" {
			WriteBadRequestError(w)
			return
		}

		if !RequireFormValue(w, r, "price") {
			return
		}

		price, err := strconv.ParseFloat(r.FormValue("price"), 64)
		if err != nil {
			WriteJsonError(w, http.StatusBadRequest,
				fmt.Sprintf("failed to parse price: %s: %v",
					r.FormValue("price"), err))
			return
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			WriteJsonError(w, http.StatusNotFound, "")
			return
		}

		switch trade.State.Status {
		case types.TradeStatusNew:
			fallthrough
		case types.TradeStatusPendingBuy:
			trade.SetLimitSellByPrice(price)
			db.DbUpdateTrade(trade)
			tradeService.BroadcastTradeUpdate(trade)
			log.WithFields(log.Fields{
				"symbol":  trade.State.Symbol,
				"tradeId": trade.State.TradeID,
				"price":   price,
			}).Info("Updated limit sell on buy.")
			return
		}

		startTime := time.Now()

		if trade.State.Status == types.TradeStatusPendingSell {
			log.Printf("Cancelling existing sell order.")
			tradeService.CancelSell(trade)
		}

		err = tradeService.LimitSellByPrice(trade, price)
		if err != nil {
			log.WithError(err).Error("Limit sell order failed.")
			WriteJsonResponse(w, http.StatusBadRequest, err.Error())
		}

		duration := time.Since(startTime)
		log.WithFields(log.Fields{
			"duration": duration,
			"symbol":   trade.State.Symbol,
		}).Debug("Sell order posted.")
	}
}

func marketSellHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		tradeId := vars["tradeId"]
		if tradeId == "" {
			WriteBadRequestError(w)
			return
		}

		trade := tradeService.FindTradeByLocalID(tradeId)
		if trade == nil {
			WriteJsonError(w, http.StatusNotFound, "")
			return
		}

		if trade.State.Status == types.TradeStatusPendingSell {
			log.Printf("Cancelling existing sell order.")
			tradeService.CancelSell(trade)
		}

		err := tradeService.MarketSell(trade, false)
		if err != nil {
			WriteJsonError(w, http.StatusInternalServerError, err.Error())
		}
	}
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	configFile := viper.ConfigFileUsed()
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Printf("error: failed to read %s: %v", configFile, err)
		return
	}
	yconf := map[interface{}]interface{}{}
	if err := yaml.Unmarshal(buf, &yconf); err != nil {
		log.Printf("error: failed to decode %s: %v", configFile, err)
		return
	}

	jconf := yaml2json(yconf)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(jconf); err != nil {
		log.WithError(err).Error("Failed to encode configuration.")
	}
}

func yaml2json(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = yaml2json(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = yaml2json(v)
		}
	}
	return i
}

func staticAssetHandler() http.HandlerFunc {
	static := packr.NewBox("../../webapp/dist")
	fileServer := http.FileServer(static)
	return func(w http.ResponseWriter, r *http.Request) {
		if !static.Has(r.URL.Path) {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}
}

func queryTradesHandler(w http.ResponseWriter, r *http.Request) {

	queryOptions := db.TradeQueryOptions{}
	queryOptions.IsClosed = true

	trades, err := db.DbQueryTrades(queryOptions)
	if err != nil {
		log.WithError(err).Error("Failed to load trades from database.")
		return
	}

	WriteJsonResponse(w, http.StatusOK, trades)
}

func getTradeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	tradeId := vars["tradeId"]
	trade, err := db.DbGetTradeByID(tradeId)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"tradeId": tradeId,
		}).Warn("Failed to find trade by ID.")
		WriteJsonResponse(w, http.StatusNotFound, "trade not found")
		return
	}
	WriteJsonResponse(w, http.StatusOK, trade)
}

func PostBuyHandler(tradeService *tradeservice.TradeService) http.HandlerFunc {
	type BuyOrderRequest struct {
		Symbol                  string              `json:"symbol"`
		Quantity                float64             `json:"quantity"`
		PriceSource             types.PriceSource   `json:"priceSource"`
		LimitSellEnabled        bool                `json:"limitSellEnabled"`
		LimitSellType           types.LimitSellType `json:"limitSellType"`
		LimitSellPercent        float64             `json:"limitSellPercent"`
		LimitSellPrice          float64             `json:"limitSellPrice"`
		StopLossEnabled         bool                `json:"stopLossEnabled"`
		StopLossPercent         float64             `json:"stopLossPercent"`
		TrailingProfitEnabled   bool                `json:"trailingProfitEnabled"`
		TrailingProfitPercent   float64             `json:"trailingProfitPercent"`
		TrailingProfitDeviation float64             `json:"trailingProfitDeviation"`
		Price                   float64             `json:"price"`
	}

	type BuyOrderResponse struct {
		TradeID string `json:"trade_id""`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		params := binance.OrderParameters{
			Side:        binance.OrderSideBuy,
			Type:        binance.OrderTypeLimit,
			TimeInForce: binance.TimeInForceGTC,
		}

		log.Printf("params: %v", log.ToJson(params))

		var requestBody BuyOrderRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&requestBody); err != nil {
			log.Printf("error: failed to decode request body: %v", err)
			WriteBadRequestError(w)
			return
		}

		log.Debugf("Received buy order request: %v", log.ToJson(requestBody))

		commonLogFields := log.Fields{
			"symbol": requestBody.Symbol,
		}

		// Validate price source.
		switch requestBody.PriceSource {
		case types.PriceSourceLast:
		case types.PriceSourceBestBid:
		case types.PriceSourceBestAsk:
		case types.PriceSourceManual:
		case "":
			WriteJsonError(w, http.StatusBadRequest, "missing required parameter: priceSource")
			return
		default:
			WriteJsonError(w, http.StatusBadRequest,
				fmt.Sprintf("invalid value for priceSource: %v", requestBody.PriceSource))
			return
		}

		// Validate limit sell.
		if requestBody.LimitSellEnabled {
			switch requestBody.LimitSellType {
			case types.LimitSellTypePercent:
			case types.LimitSellTypePrice:
			default:
				WriteJsonError(w, http.StatusBadRequest,
					fmt.Sprintf("limit sell type invalid or not set"))
				return
			}
		}

		params.Symbol = requestBody.Symbol
		params.Quantity = requestBody.Quantity

		orderId, err := tradeService.MakeOrderID()
		if err != nil {
			log.WithFields(commonLogFields).WithError(err).Errorf("Failed to create order ID.")
			WriteJsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		params.NewClientOrderId = orderId

		trade := types.NewTrade()
		trade.AddHistory(types.HistoryEntry{
			Timestamp: time.Now(),
			Type: types.HistoryTypeCreated,
			Fields: requestBody,
		})
		trade.State.Symbol = params.Symbol
		trade.AddClientOrderID(params.NewClientOrderId)

		buyService := binanceex.NewBinancePriceService()

		switch requestBody.PriceSource {
		case types.PriceSourceManual:
			params.Price = requestBody.Price
		default:
			params.Price, err = buyService.GetPrice(params.Symbol, requestBody.PriceSource)
			if err != nil {
				log.WithError(err).WithFields(commonLogFields).WithFields(log.Fields{
					"priceSource": requestBody.PriceSource,
				}).Error("Failed to get buy price.")
				WriteJsonError(w, http.StatusInternalServerError,
					fmt.Sprintf("Failed to get price: %v", err))
				return
			}
		}

		if requestBody.StopLossEnabled {
			trade.SetStopLoss(requestBody.StopLossEnabled,
				requestBody.StopLossPercent)
		}

		if requestBody.TrailingProfitEnabled {
			trade.SetTrailingProfit(requestBody.TrailingProfitEnabled,
				requestBody.TrailingProfitPercent,
				requestBody.TrailingProfitDeviation)
		}

		tradeId := tradeService.AddNewTrade(trade)
		commonLogFields["tradeId"] = tradeId;

		if requestBody.LimitSellEnabled {
			if requestBody.LimitSellType == types.LimitSellTypePercent {
				log.WithFields(commonLogFields).Infof("Setting limit sell at %f percent.",
					requestBody.LimitSellPercent)
				trade.SetLimitSellByPercent(requestBody.LimitSellPercent)
			} else if requestBody.LimitSellType == types.LimitSellTypePrice {
				log.WithFields(commonLogFields).Infof("Setting limit sell at price %f.",
					requestBody.LimitSellPrice)
				trade.SetLimitSellByPrice(requestBody.LimitSellPrice)
			}
		}

		log.WithFields(commonLogFields).WithFields(log.Fields{
			"type":                    params.Type,
			"price":                   params.Price,
			"quantity":                params.Quantity,
			"clientOrderId":           params.NewClientOrderId,
			"priceSource":             requestBody.PriceSource,
			"limitSellEnabled":        requestBody.LimitSellEnabled,
			"limitSellType":           requestBody.LimitSellType,
			"limitSellPercent":        requestBody.LimitSellPercent,
			"limitSellPrice":          requestBody.LimitSellPrice,
			"stopLossEnabled":         requestBody.StopLossEnabled,
			"stopLossPercent":         requestBody.StopLossPercent,
			"trailingProfitEnabled":   requestBody.TrailingProfitEnabled,
			"trailingProfitPercent":   requestBody.TrailingProfitPercent,
			"trailingProfitDeviation": requestBody.TrailingProfitDeviation,
		}).Infof("Posting BUY order for %s", params.Symbol)

		response, err := binanceex.GetBinanceRestClient().PostOrder(params)
		if err != nil {
			log.WithError(err).
				Errorf("Failed to post buy order.")
			switch err := err.(type) {
			case *binance.RestApiError:
				log.Debugf("Forwarding Binance error repsonse.")
				w.WriteHeader(response.StatusCode)
				w.Write(err.Body)
			default:
				WriteJsonResponse(w, http.StatusInternalServerError,
					err.Error())
			}
			if trade != nil {
				tradeService.FailTrade(trade)
			}
			return
		}

		data, err := ioutil.ReadAll(response.Body)
		var buyResponse binance.PostOrderResponse
		if err := json.Unmarshal(data, &buyResponse); err != nil {
			log.Printf("error: failed to decode buy order response: %v", err)
		}
		log.WithFields(log.Fields{
			"tradeId": tradeId,
		}).Debugf("Decoded BUY response: %s", log.ToJson(buyResponse))

		WriteJsonResponse(w, http.StatusOK, BuyOrderResponse{
			TradeID: tradeId,
		})
	}
}
