package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	. "strconv"
	"strings"
	"time"
	//	"sync"
	//"os"
)

// Is Charging lcctc
// 1. Fix Error Handling
// 2. Increase Check Last State
// termianl menu interface
// currently data
// option to change data

func main() {
	interval := 5
	updateTimer := time.Duration(1 * time.Second)
	warnTimer := time.Duration(10 * time.Second)
	timeoutTimer := time.Duration(time.Minute * 10)
	lastCheck := newCheckLastNegativePower(false)
	egoUrls := NewStructEgoData()
	statusUrl := egoUrls.status("amp", "psm", "frc", "lcctc", "alw")
	lastPowerOn := time.Now()
	for {
		start := time.Now()
		forever := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		sleep := time.After(updateTimer)
		warn := time.After(warnTimer)
		timeout := time.After(timeoutTimer)
		//	var exitWarn sync.WaitGroup
		//	tx1 := make(chan None)
		//	tx4 := make(chan None)
		tx2 := make(chan EgoStatus)
		tx3 := make(chan []DiscovergyData)
		timeInt := TimeAtUnix(interval)
		go getEgoStatus(statusUrl, tx2)
		go ParseDiscovergy(timeInt, tx3)
		go func(ctx context.Context) {
			select {
			case <-ctx.Done():
				forever <- struct{}{}
				return
			case <-warn:
				for {
					select {
					case <-ctx.Done():
						forever <- struct{}{}
						return
					default:
						log.Printf("Warnung: Kein Daten seit %v. Abschaltung in %v\n", time.Since(start), timeoutTimer-time.Since(start))
						time.Sleep(5 * time.Second)
					}
				}
			}

		}(ctx)
		go func(ctx context.Context) {
			for {
				select {
				case <-ctx.Done():
					forever <- struct{}{}
					return
				case <-timeout:
					log.Fatal("Timeout")
				}
			}
		}(ctx)
		dGyData := <-tx3
		egoStatusStruct := <-tx2
		//fmt.Println("Befor Parse", time.Since(start))
		MeasureData(dGyData, egoStatusStruct, &lastCheck, egoUrls, &lastPowerOn)
		//	fmt.Println("After", time.Since(start))
		cancel()
		<-forever
		<-sleep
		//fmt.Println("End", time.Since(start))
	}
}

type CheckLastNegativePower struct {
	beginCheck   time.Time
	checkCurrent bool
}

func newCheckLastNegativePower(state bool) CheckLastNegativePower {
	return CheckLastNegativePower{
		beginCheck:   time.Now(),
		checkCurrent: state,
	}
}
func (cl *CheckLastNegativePower) stopCheck() {
	*cl = newCheckLastNegativePower(false)
}
func (cl *CheckLastNegativePower) checkNow() {
	*cl = newCheckLastNegativePower(true)
}
func TimeAtUnix(interval int) int {
	intervalDuration := time.Duration(interval)
	timeInterval := time.Duration(intervalDuration * time.Minute)
	timeUnix := time.Now().Add(-timeInterval)
	//	fmt.Println("Unix Milli: ", timeUnix.UnixMilli(), "\nFormat:", timeUnix)
	return TimeToInt(timeUnix)
}
func TimeToInt(t time.Time) int {
	return int(t.UnixMilli())
}
func EgoUrlIsEmpty(baseUrl, curUrl string) bool {
	return baseUrl == curUrl
}
func MeasureData(dGyData []DiscovergyData, egoStatus EgoStatus, lastNegative *CheckLastNegativePower, egoUrls StructEgoData, powerOn *time.Time) {
	if len(dGyData) == 0 {
		log.Fatal("Keine Daten von Discovergy")
	}
	min := CalculateChangeOfAmpere(dGyData[0].Values)
	for _, val := range dGyData {
		change := CalculateChangeOfAmpere(val.Values)
		//newPower := egoStatus.Amp + change
		// n Minuten Minimum zum Hochschalten
		if change < min {
			min = change
		}
	}

	// Frc 1 Off 2 On 0 Auto
	lastPower := CalculateChangeOfAmpere(dGyData[len(dGyData)-1].Values)
	//fmt.Printf("letzter: %v, min %v\n", lastPower, min)
	//fmt.Println(lastPower, min)
	egoSetBaseUrl := egoUrls.EGO_URL_SET + "?"
	queryList := []string{}
	DecreasePower(lastPower, egoStatus, &queryList)
	IncreasePower(min, egoStatus, &queryList, powerOn)
	TurnOnPower(min, egoStatus, &queryList, powerOn)
	TurnOffPower(lastPower, egoStatus, lastNegative, dGyData[len(dGyData)-1], &queryList)
	egoUrlSetString := MakeEgoUrlSet(egoSetBaseUrl, queryList)
	if !EgoUrlIsEmpty(egoSetBaseUrl, egoUrlSetString) {
		fmt.Println(time.Now())
		fmt.Println("Set => ", egoUrlSetString)
		msg, err := MakeGetRequest(NewRequestConfig(egoUrlSetString))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(msg)
	}
	//fmt.Println(egoStatus.Alw)
}
func checkPowerMin(newPower int64) int64 {
	if newPower > 6 {
		return newPower
	}
	return 6
}
func (es EgoStatus) curEqNewPower(newPower int64) bool {
	return es.Amp == newPower

}

// Stromstäkre auf 16A regulieren
func checkPowerMax(newPower int64) int64 {
	if newPower < 16 {
		return newPower
	}
	return 16
}
func DecreasePower(powerNet int64, es EgoStatus, queryList *[]string) {
	// Sofort runterschalten wenn Unterschuss,
	// jedoch mindestens auf 6A
	if powerNet >= 0 {
		return
	}
	if !es.Alw {
		return
	}
	newPower := checkPowerMin(powerNet + es.Amp)
	if es.curEqNewPower(newPower) {
		return
	}
	fmt.Println("Runtergeschaltet auf", newPower, "um", time.Now())
	EgoUrlSetUpdate(queryList, "amp", Itoa(int(newPower)))
}
func (a *CheckLastNegativePower) CheckIfMinimumReached(currentPower int64, diGyData DiscovergyData) {
	if int(diGyData.Time) > TimeToInt(a.beginCheck) && currentPower >= 6 {
		a.stopCheck()
	}
}
func (a *CheckLastNegativePower) CheckIfBelowMinimum(currentPower int64, egoStatus EgoStatus) {
	//fmt.Println(egoStatus.Frc)
	if !a.checkCurrent && currentPower < 6 && egoStatus.Alw {
		a.checkNow()
	}
}
func (a CheckLastNegativePower) TimeExceeded() bool {
	return time.Since(a.beginCheck) > time.Duration(5*time.Minute) && a.checkCurrent
}
func IncreasePower(powerNet int64, es EgoStatus, queryList *[]string, lastPoweredOn *time.Time) {
	// Stromstärke nicht öndern, wenn n Minuten Minimium der alten Stromstärke entstrpicht
	// oder keinen Überschuss gibt.
	if time.Since(*lastPoweredOn) < 5*time.Minute {
		return
	}
	if powerNet <= 0 {
		return
	}
	if !es.Alw {
		return
	}

	// n Minuten Minimum + aktuelle Stromstörke ist die neue Stromstärke.
	powerMin := checkPowerMax(powerNet + es.Amp)
    fmt.Println(powerMin)
	if es.curEqNewPower(powerMin) {
		return
	}
	// Ampere auf n Minuten Minimium setzen.
	fmt.Println("Hochgeschaltet auf", powerMin, "um", time.Now())
	EgoUrlSetUpdate(queryList, "amp", Itoa(int(powerMin)))
}
func TurnOnPower(powerNet int64, es EgoStatus, queryList *[]string, powerOn *time.Time) {
	powerMin := checkPowerMax(powerNet + es.Amp)
	//fmt.Printf("TurnOnPower: %v\n", powerMin)
	if powerMin < 6 {
		return
	}
	// Anschalten wenn n Minuten Überschuss
	// Frc: 0 Auto, 1 Aus, 2 An
	if !es.Alw {
		fmt.Println("Angeschaltet um ", time.Now())
		fmt.Printf("aktuell niedrigste Stromstärke: %v\n", powerMin)
		EgoUrlSetUpdate(queryList, "frc", "2")
		*powerOn = time.Now()
	}
}
func CurrentStatus() {

}

func TurnOffPower(powerNet int64, es EgoStatus, ln *CheckLastNegativePower, lastPower DiscovergyData, queryList *[]string) {
	totalPower := powerNet + es.Amp
	ln.CheckIfBelowMinimum(totalPower, es)
	if !ln.checkCurrent {
		return
	}
	/*if es.Frc == 1 {
		return
	}
	*/
	ln.CheckIfMinimumReached(totalPower, lastPower)
	if ln.TimeExceeded() {
		fmt.Println("Ausgeschaltet um", time.Now())
		fmt.Printf("aktuelle Stromstärke: %v\n", totalPower)
		EgoUrlSetUpdate(queryList, "frc", "1")
		ln.stopCheck()
	}
}
func CalculateChangeOfAmpere(dGyPowData DiscovergyPowerData) int64 {
	changeOfAmpere := -(float64(dGyPowData.Power) / float64(dGyPowData.Phase1Voltage))
	return int64(math.Floor(changeOfAmpere))
}
func MakeDiGyUrl(timeAt int) string {
	return fmt.Sprintf("%s/readings?meterId=%s&from=%d&resultion=raw",
		BASE_URL, METER_ID, timeAt)
}

func ParseDiscovergy(timeAt int, tx3 chan []DiscovergyData) {
	login := NewUserLogin(USERNAME, PASSWD)
	url := MakeDiGyUrl(timeAt)
	requestConfig := NewRequestConfig(url).WithUserLogin(login)
	dataStr := HandleGetRequest(requestConfig, time.Minute*10, MakeGetRequestBasicAuth)
	dataStruct := DiscovergyDataCollection{}
	json.Unmarshal([]byte(dataStr), &dataStruct.Collection)
	tx3 <- dataStruct.Collection
}

type DiscovergyData struct {
	Time   int64
	Values DiscovergyPowerData
}

type DiscovergyDataCollection struct {
	Collection []DiscovergyData
}

type DiscovergyPowerData struct {
	Power         int64
	Phase1Power   int64
	Phase2Power   int64
	Phase3Power   int64
	Phase1Voltage int64
	Phase2Voltage int64
	Phase3Voltage int64
	Energy        int64
	EnergyOut     int64
	Energy1       int64
	Energy2       int64
}
type StructEgoData struct {
	EGO_URL_API    string
	EGO_URL_STATUS string
	EGO_URL_SET    string
	ExtractedData  EgoStatus
}

func NewStructEgoData() StructEgoData {
	EGO_URL_API := EGO_URL + "api/"
	EGO_URL_STATUS := EGO_URL_API + "status"
	EGO_URL_SET := EGO_URL_API + "set"
	var egoData EgoStatus
	return StructEgoData{
		EGO_URL_API,
		EGO_URL_STATUS,
		EGO_URL_SET,
		egoData,
	}
}
func (ego *StructEgoData) status(querys ...string) string {
	base := ego.EGO_URL_STATUS + "?filter="
	return base + strings.Join(querys, ",")
}

type EgoStatus struct {
	Amp, Psm, Frc int64
	Lcctc         float64
	Alw           bool
}

func getEgoStatus(url string, tx2 chan EgoStatus) {
	statusStr := HandleGetRequest(NewRequestConfig(url), time.Minute*10, MakeGetRequest)
	egoStatus := EgoStatus{}
	json.Unmarshal([]byte(statusStr), &egoStatus)
	tx2 <- egoStatus
}

type EgoUrlSet struct {
	Url string
}

func newEgoUrlSet(url string) EgoUrlSet {
	return EgoUrlSet{url}
}
func (a *EgoUrlSet) Update(suffix, value string) {
	*a = newEgoUrlSet(fmt.Sprintf("%s&%s=%s", a.Url, suffix, value))
}
func EgoUrlSetUpdate(queryList *[]string, suffix, value string) {
	*queryList = append(*queryList, fmt.Sprintf("%s=%s", suffix, value))
}
func MakeEgoUrlSet(baseUrlSet string, queryList []string) string {
	return baseUrlSet + strings.Join(queryList, "&")
}
