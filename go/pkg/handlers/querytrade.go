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

package handlers

import (
	"net/http"
	"gitlab.com/crankykernel/maker/pkg/db"
	"gitlab.com/crankykernel/maker/pkg/log"
)

func QueryTrades(w http.ResponseWriter, r *http.Request) {

	queryOptions := db.TradeQueryOptions{}
	queryOptions.IsClosed = true

	trades, err := db.DbQueryTrades(queryOptions)
	if err != nil {
		log.WithError(err).Error("Failed to load trades from database.")
		return
	}

	WriteJsonResponse(w, http.StatusOK, trades)
}