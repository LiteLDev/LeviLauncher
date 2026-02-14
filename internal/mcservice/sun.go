package mcservice

import (
	"fmt"
	"net/http"
	"time"

	json "github.com/goccy/go-json"
)

type SunTimes struct {
	Sunrise string `json:"sunrise"`
	Sunset  string `json:"sunset"`
	IP      string `json:"ip"`
}

func GetSunTimes() SunTimes {
	resp, err := http.Get("http://ip-api.com/json")
	if err != nil {
		return SunTimes{Sunrise: "06:00", Sunset: "18:00"}
	}
	defer resp.Body.Close()

	var geo struct {
		Status    string  `json:"status"`
		IP        string  `json:"query"`
		Longitude float64 `json:"lon"`
		Latitude  float64 `json:"lat"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&geo); err != nil || geo.Status != "success" {
		return SunTimes{Sunrise: "06:00", Sunset: "18:00"}
	}

	apiURL := fmt.Sprintf("https://api.sunrise-sunset.org/json?lat=%f&lng=%f&formatted=0", geo.Latitude, geo.Longitude)
	sunResp, err := http.Get(apiURL)
	if err != nil {
		return SunTimes{Sunrise: "06:00", Sunset: "18:00", IP: geo.IP}
	}
	defer sunResp.Body.Close()

	var sunData struct {
		Results struct {
			Sunrise string `json:"sunrise"`
			Sunset  string `json:"sunset"`
		} `json:"results"`
		Status string `json:"status"`
	}

	if err := json.NewDecoder(sunResp.Body).Decode(&sunData); err != nil || sunData.Status != "OK" {
		return SunTimes{Sunrise: "06:00", Sunset: "18:00", IP: geo.IP}
	}

	parseAndConvert := func(utcStr string) string {
		t, err := time.Parse(time.RFC3339, utcStr)
		if err != nil {
			return "00:00"
		}
		localTime := t.Local()
		return localTime.Format("15:04")
	}

	return SunTimes{
		Sunrise: parseAndConvert(sunData.Results.Sunrise),
		Sunset:  parseAndConvert(sunData.Results.Sunset),
		IP:      geo.IP,
	}
}

