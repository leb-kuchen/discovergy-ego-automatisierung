package main

import (
	"net/http"
	"log"
	"io/ioutil"
	"fmt"
	"time"
//	"strconv"
	"encoding/json"
	"math"
	"strings"
)

// termianl menu interface
// currently data
// option to change data

func main() {
	interval := 1
	timeInt := TimeAtUnix(interval)
	lastCheck := newCheckLastNegativePower(false)
	fmt.Println(lastCheck)
	res := basicAuth(timeInt)
	//fmt.Println(res)
	egoUrls :=  NewStructEgoData()
	statusUrl := egoUrls.status("amp", "psm", "frc")
	egoStatusStruct := getEgoStatus(statusUrl)
	fmt.Println(egoUrls)
	dGyData := ParseDiscovergy(res)
	MeasureData(dGyData, egoStatusStruct, &lastCheck)
}
type CheckLastNegativePower struct{
	beginCheck time.Time
	checkCurrent bool
}
func newCheckLastNegativePower(state bool) CheckLastNegativePower {
	return CheckLastNegativePower{
		beginCheck: time.Now(),
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
// Anschalten Logik
func MeasureData(dGyData []DiscovergyData, egoStatus EgoStatus, lastNegative *CheckLastNegativePower) {
	min := CalculateChangeOfAmpere(dGyData[0].Values)
	for _, val := range dGyData {
		change := CalculateChangeOfAmpere(val.Values)
		newPower := egoStatus.Amp + change
		if change < min {
			min = change
		}
		// if fsr on 
		if newPower < 6 && !lastNegative.checkCurrent{
			lastNegative.checkNow() 
			fmt.Println("IN FÜNF MINUTEN RUNTERSCHALTEN")
		}
		if lastNegative.checkCurrent {
			if newPower >= 6 && val.Time > int64(TimeToInt(lastNegative.beginCheck)){
				lastNegative.stopCheck()
			} else if time.Since(lastNegative.beginCheck) > time.Duration(5 * time.Minute) {
				fmt.Println("AUSCHALTEN")
			}
		}
		fmt.Println(lastNegative.checkCurrent, lastNegative.beginCheck)
		if change < 0{
			fmt.Println("SOFORT RUNTERSCHALTEN", change, newPower)
		}
	
	//	fmt.Println("amp", egoStatus.Amp, "psm", egoStatus.Psm, "newPower", newPower)
		//fmt.Println(change, min)
	}
	// Frc 1 Off 2 On 0 Auto
	if min > 0 {
		fmt.Println("HOCHSCHALTEN", min)
		if egoStatus.Frc == 1 {
			fmt.Println("Anschalten")
		}
				
	}
	fmt.Println(egoStatus.Amp + min)
}
// 0 check failed stop
// 1 check failed continue
// 2 check succesful
func (ln *CheckLastNegativePower) doCheck(newPower int64) int64{
	if !(newPower <= 6) {
		return 2
	}	
	if !ln.checkCurrent{
		return 2
	}
	if time.Since(ln.beginCheck) > time.Duration(5 * time.Minute) {
		return 0
	}
	return 1
}
func CalculateChangeOfAmpere(dGyPowData DiscovergyPowerData) int64{
	changeOfAmpere := -(float64(dGyPowData.Power) / float64(dGyPowData.Phase1Voltage))
	return int64(math.Floor(changeOfAmpere))
}
func basicAuth(timeAt int) string {
	fmt.Println("TimeAt", timeAt)
    url := fmt.Sprintf("%s/readings?meterId=%s&from=%d&resultion=raw",
    BASE_URL, METER_ID, timeAt)
    client := &http.Client{}
    req, err := http.NewRequest("GET", url, nil)
    req.SetBasicAuth(USERNAME, PASSWD)
    resp, err := client.Do(req)
    if err != nil{
        log.Fatal(err)
    }
    bodyText, err := ioutil.ReadAll(resp.Body)
    if err != nil{
    	log.Fatal(err)
    }
    fmt.Println("Done")
    return string(bodyText)
   
}
func ParseDiscovergy(dataStr string) []DiscovergyData {
	dataStruct := DiscovergyDataCollection{}
	json.Unmarshal([]byte(dataStr), &dataStruct.Collection)
	return dataStruct.Collection
}
type DiscovergyData struct {
	Time int64
	Values DiscovergyPowerData	
}

type DiscovergyDataCollection struct {
	Collection []DiscovergyData
}

type DiscovergyPowerData struct {
	Power int64
	Phase1Power int64
	Phase2Power int64
	Phase3Power int64
	Phase1Voltage int64
	Phase2Voltage int64
	Phase3Voltage int64
	Energy int64
	EnergyOut int64
	Energy1 int64
	Energy2 int64
}
type StructEgoData struct {
	EGO_URL_API string
	EGO_URL_STATUS string
	EGO_URL_SET string
	ExtractedData EgoStatus
}
func NewStructEgoData()  StructEgoData {
	EGO_URL_API :=  EGO_URL + "api/"
	EGO_URL_STATUS := EGO_URL_API + "status"
	EGO_URL_SET := EGO_URL_API + "set"
	var egoData EgoStatus
	return StructEgoData {
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
// Logik hoch dann runterschalten 
// var LastCheckIncreasePower 

type EgoStatus struct {
	Amp, Psm, Frc int64
}
func getEgoStatus(url string) EgoStatus {
	statusStr := MakeGetRequest(url)
	egoStatus := EgoStatus{}
	json.Unmarshal([]byte(statusStr), &egoStatus)
	return egoStatus
}

func GetEgoUrl(prefix, suffix, value string) string {
	return fmt.Sprintf("%s?%s=%s", prefix, suffix, value)
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


