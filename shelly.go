package main

import (
	"fmt"
	"time"
)

type Shelly struct {
	BaseUrl string
	//ShellyJson
}

func NewShelly(s string) Shelly {
	return Shelly{
		BaseUrl: s,
	}
}

type PowerDevice interface {
	GetBaseUrl() string
}
type ShellyJson struct {
	MaxPower int
}

func QueryFromBaseUrl(a PowerDevice, new string) string {
	return fmt.Sprintf("%v?%v", a.GetBaseUrl(), new)
}

// 2217 merken
func (a Shelly) GetBaseUrl() string {
	return a.BaseUrl
}
func (a Shelly) TurnOn(d DiscovergyPowerData) bool {
	return (d.Power / 1000) < int64(-a.PowerDraw())
}
func (a Shelly) TurnOff(d DiscovergyPowerData) bool {
	return CalculateChangeOfAmpere(d) < 0
}
func (a Shelly) MakeRequest(d DiscovergyPowerData) {
	url := a.TurnUrl(d)
	if url != "" {
		HandleGetRequest(NewRequestConfig(url), 10*time.Minute, MakeGetRequest)
	}
}
func (a Shelly) TurnUrl(d DiscovergyPowerData) string {
	if a.TurnOn(d) {
		QueryFromBaseUrl(a, "turn=on")
		fmt.Println("Shelly an")
	} else if a.TurnOff(d) {
		QueryFromBaseUrl(a, "turn=off")
		fmt.Println("Shelly aus")
	}
	return ""
}
func (a Shelly) PowerDraw() uint {
	return 2217
}
