package main

import (
	"fmt"
	//"strings"
)

type Ego struct {
	BaseUrl string
}

func newEgo() Ego {
	return Ego{
		BaseUrl: EGO_URL,
	}
}
func (a Ego) TurnOnPower() string {
	return "frc=2"
}
func (a Ego) TurnOffPower() string {
	return "frc=1"
}
func (a Ego) SetPower(amp int) string {
	return fmt.Sprintf("amp=%v", amp)
}
func (a Ego) Url(p string) string {
	switch p {
	case "base":
	case "status":
	case "set":

	}
	return ""
}
