package handlers

import (
	"encoding/json"
	"eth2-exporter/db"
	"eth2-exporter/price"
	"eth2-exporter/services"
	"eth2-exporter/templates"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// Blocks will return information about blocks using a go template
func GasNow(w http.ResponseWriter, r *http.Request) {
	var gasNowTemplate = templates.GetTemplate("layout.html", "gasnow.html")

	w.Header().Set("Content-Type", "text/html")

	data := InitPageData(w, r, "gasnow", "/gasnow", fmt.Sprintf("%v Gwei", 34))

	now := time.Now().Truncate(time.Minute)
	lastWeek := time.Now().Truncate(time.Minute).Add(-time.Hour * 24 * 7)

	history, err := db.BigtableClient.GetGasNowHistory(now, lastWeek)
	if err != nil {
		logger.Errorf("error retrieving gas price histors: %v", err)
		return
	}

	group := make(map[int64]float64, 0)
	for i := 0; i < len(history); i++ {
		_, ok := group[history[i].Ts.Truncate(time.Hour).Unix()]
		if !ok {
			group[history[i].Ts.Truncate(time.Hour).Unix()] = float64(history[i].Fast.Int64())
		} else {
			group[history[i].Ts.Truncate(time.Hour).Unix()] = (group[history[i].Ts.Truncate(time.Hour).Unix()] + float64(history[i].Fast.Int64())) / 2
		}
	}

	resRet := []*struct {
		Ts      int64   `json:"ts"`
		AvgFast float64 `json:"fast"`
	}{}

	for ts, fast := range group {
		resRet = append(resRet, &struct {
			Ts      int64   `json:"ts"`
			AvgFast float64 `json:"fast"`
		}{
			Ts:      ts,
			AvgFast: fast,
		})
	}

	sort.SliceStable(resRet, func(i int, j int) bool {
		return resRet[i].Ts > resRet[j].Ts
	})

	data.Data = resRet

	if handleTemplateError(w, r, gasNowTemplate.ExecuteTemplate(w, "layout", data)) != nil {
		return // an error has occurred and was processed
	}
}

func GasNowData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	gasnowData := services.LatestGasNowData()
	if gasnowData == nil {
		logger.Error("error obtaining latest gas now data 'nil'")
		http.Error(w, "Internal server error", http.StatusServiceUnavailable)
		return
	}

	currency := GetCurrency(r)
	if currency == "ETH" {
		currency = "USD"
	}
	gasnowData.Data.Price = price.GetEthPrice(currency)
	gasnowData.Data.Currency = currency

	err := json.NewEncoder(w).Encode(gasnowData)
	if err != nil {
		logger.Errorf("error serializing json data for API %v route: %v", r.URL.String(), err)
		http.Error(w, "Internal server error", http.StatusServiceUnavailable)
		return
	}
}
