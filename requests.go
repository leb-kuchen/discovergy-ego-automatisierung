package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func MakeGetRequest(rc RequestConfig) (string, error) {
	resp, err := http.Get(rc.Url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
func HandleGetRequest(rc RequestConfig, timer time.Duration, rf func(RequestConfig) (string, error)) string {
	timeout := time.After(timer)
	bodyStr, err := rf(rc)
	for ; err != nil; bodyStr, err = rf(rc) {
		select {
		case <-timeout:
			log.Fatalf("Timout nach %v beim wiederholten Abfragen der Url %q.\nDetailierte Fehlermelung: %v", timer, rc.Url, err)
		default:
			log.Printf("Fehler beim Abragen der Url %q. Anfrage wird erneut versucht.\n", rc.Url)
		}
		time.Sleep(time.Second * 5)
	}
	return bodyStr
}
func MakeGetRequestBasicAuth(rc RequestConfig) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", rc.Url, nil)
	ul := rc.UserLogin
	req.SetBasicAuth(ul.Username, ul.Password)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyText), nil
}

type UserLogin struct {
	Username string
	Password string
}

func NewUserLogin(username, password string) UserLogin {
	return UserLogin{
		username,
		password,
	}
}

type RequestConfig struct {
	UserLogin UserLogin
	Url       string
}

func NewRequestConfig(url string) RequestConfig {
	return RequestConfig{
		NewUserLogin("", ""),
		url,
	}
}
func (a RequestConfig) WithUserLogin(ul UserLogin) RequestConfig {
	a.UserLogin = ul
	return a
}
