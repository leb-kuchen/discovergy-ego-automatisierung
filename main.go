package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	. "strconv"
	"strings"
	"time"
)

// termianl menu interface
// currently data
// option to change data

func main() {
	interval := 5
	updateTimer := time.Duration(5 * time.Second)
	lastCheck := newCheckLastNegativePower(false)
	egoUrls := NewStructEgoData()
	statusUrl := egoUrls.status("amp", "psm", "frc")
	for {
		tx1 := make(chan int)
		tx2 := make(chan EgoStatus)
		tx3 := make(chan string)
		timeInt := TimeAtUnix(interval)
		//	start := time.Now()
		go func() {
			time.Sleep(updateTimer)
			tx1 <- 0
		}()
		go basicAuth(timeInt, tx3)
		go getEgoStatus(statusUrl, tx2)
		res := <-tx3
		egoStatusStruct := <-tx2
		//fmt.Println("Befor Parse", time.Since(start))
		dGyData := ParseDiscovergy(res)
		MeasureData(dGyData, egoStatusStruct, &lastCheck, egoUrls)
		//	fmt.Println("After", time.Since(start))
		<-tx1
		//	fmt.Println("End", time.Since(start))

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
func TimeToInt(time time.Time) int {
	return int(time.UnixMilli())
}
func EgoUrlIsEmpty(baseUrl, curUrl string) bool {
	return baseUrl == curUrl
}
func MeasureData(dGyData []DiscovergyData, egoStatus EgoStatus, lastNegative *CheckLastNegativePower, egoUrls StructEgoData) {
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
	//fmt.Println(lastPower, min)
	egoSetBaseUrl := egoUrls.EGO_URL_SET + "?"
	queryList := []string{}
	DecreasePower(lastPower, egoStatus, &queryList)
	IncreasePower(min, egoStatus, &queryList)
	TurnOnPower(min, egoStatus, &queryList)
	TurnOffPower(lastPower, egoStatus, lastNegative, dGyData[len(dGyData)-1], &queryList)
	egoUrlSetString := MakeEgoUrlSet(egoSetBaseUrl, queryList)
	if !EgoUrlIsEmpty(egoSetBaseUrl, egoUrlSetString) {
		fmt.Println(time.Now())
		fmt.Println("Set => ", egoUrlSetString)
		MakeGetRequest(egoUrlSetString)
	}
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
	if !a.checkCurrent && currentPower < 6 && egoStatus.Frc != 1 {
		a.checkNow()
	}
}
func (a CheckLastNegativePower) TimeExceeded() bool {
	return time.Since(a.beginCheck) > time.Duration(5*time.Minute) && a.checkCurrent
}
func IncreasePower(powerNet int64, es EgoStatus, queryList *[]string) {
	// Stromstärke nicht öndern, wenn n Minuten Minimium der alten Stromstärke entstrpicht
	// oder keinen Überschuss gibt.
	if powerNet <= 0 {
		return
	}
	// n Minuten Minimum + aktuelle Stromstörke ist die neue Stromstärke.
	powerMin := checkPowerMax(powerNet + es.Amp)
	if es.curEqNewPower(powerMin) {
		return
	}
	// Ampere auf n Minuten Minimium setzen.
	fmt.Println("Hochgeschaltet auf", powerMin, "um", time.Now())
	EgoUrlSetUpdate(queryList, "amp", Itoa(int(powerMin)))
}
func TurnOnPower(powerNet int64, es EgoStatus, queryList *[]string) {
	powerMin := checkPowerMax(powerNet + es.Amp)
	if powerMin < 6 {
		return
	}
	// Anschalten wenn n Minuten Überschuss
	// Frc: 0 Auto, 1 Aus, 2 An
	switch es.Frc {
	case 0, 1:
		fmt.Println("Angeschaltet um ", time.Now())
		fmt.Printf("aktuell niedrigste Stromstärke: %v\n", powerMin)
		EgoUrlSetUpdate(queryList, "frc", "2")
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
func basicAuth(timeAt int, tx3 chan string) {
	url := fmt.Sprintf("%s/readings?meterId=%s&from=%d&resultion=raw",
		BASE_URL, METER_ID, timeAt)
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(USERNAME, PASSWD)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	tx3 <- string(bodyText)

}
func ParseDiscovergy(dataStr string) []DiscovergyData {
	dataStruct := DiscovergyDataCollection{}
	json.Unmarshal([]byte(dataStr), &dataStruct.Collection)
	return dataStruct.Collection
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
}

func getEgoStatus(url string, tx2 chan EgoStatus) {
	statusStr := MakeGetRequest(url)
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
func MakeGetRequest(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	return string(body)
}
