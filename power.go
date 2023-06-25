package main

import (
	"strings"
)

type PowerDevice interface {
	TurnOnPower() string
	TurnOffPower() string
	IncreasePower() string
	DecreasePower() string
	Url(string) string
}

func CreateRequestUrl(a PowerDevice, urlType string, params []string) string {
	return a.Url(urlType) + "?" + strings.Join(params, "&")
}
