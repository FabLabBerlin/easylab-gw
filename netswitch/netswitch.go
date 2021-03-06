/*
netswitch control.

The NetSwitches have Relays that must be in sync with our system.  For example
when a NetSwitch is plugged in, the relay must go into the correct position.
The mfi Switches are by default switched on and need to turn off after being
plugged in.
*/
package netswitch

import (
	"encoding/json"
	"fmt"
	"github.com/FabLabBerlin/easylab-gw/global"
	"github.com/FabLabBerlin/easylab-lib/mfi"
	"github.com/FabLabBerlin/easylab-lib/xmpp"
	"github.com/FabLabBerlin/easylab-lib/xmpp/commands"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const NETSWITCH_TYPE_MFI = "mfi"

type NetSwitch struct {
	muChInit sync.Mutex
	chSingle chan int
	On       bool `json:"-"`
	// We're using this without Beego ORM attached
	Id                  int64
	NetswitchUrlOn      string
	NetswitchUrlOff     string
	NetswitchHost       string
	NetswitchSensorPort int
	NetswitchType       string
}

func (ns *NetSwitch) SetOn(on bool) (err error) {
	if on {
		return ns.turnOn()
	} else {
		return ns.turnOff()
	}
}

func (ns *NetSwitch) turnOn() (err error) {
	log.Printf("turn on %v", ns.UrlOn())
	var resp *http.Response
	if ns.NetswitchType == NETSWITCH_TYPE_MFI {
		resp, err = http.PostForm(ns.UrlOn(), url.Values{"output": {"1"}})
	} else {
		resp, err = http.Get(ns.UrlOn())
	}
	if ns.isIgnorableError(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("http: %v", err)
	}
	if resp == nil {
		log.Printf("turnOn: resp is nil!")
	} else {
		if resp.Body != nil {
			defer resp.Body.Close()
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
		}
	}
	ns.On = true
	return
}

func (ns *NetSwitch) turnOff() (err error) {
	var resp *http.Response
	if ns.NetswitchType == NETSWITCH_TYPE_MFI {
		log.Printf("turn off %v", ns.UrlOn())
		resp, err = http.PostForm(ns.UrlOn(), url.Values{"output": {"0"}})
	} else {
		log.Printf("turn off %v", ns.UrlOff())
		resp, err = http.Get(ns.UrlOff())
	}
	if ns.isIgnorableError(err) {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("http: %v", err)
	}
	if resp == nil {
		log.Printf("turnOff: resp is nil!")
	} else {
		if resp.Body != nil {
			defer resp.Body.Close()
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
		}
	}
	ns.On = false
	return
}

func (ns *NetSwitch) isIgnorableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "malformed HTTP status code") &&
		strings.Contains(msg, "maSwitch")
}

func (ns *NetSwitch) UrlOn() string {
	if ns.NetswitchType == NETSWITCH_TYPE_MFI {
		return "http://" + ns.NetswitchHost + "/sensors/" + strconv.Itoa(ns.NetswitchSensorPort)
	} else {
		return ns.NetswitchUrlOn
	}
}

func (ns *NetSwitch) UrlOff() string {
	if ns.NetswitchType == NETSWITCH_TYPE_MFI {
		return "http://" + ns.NetswitchHost + "/sensors/" + strconv.Itoa(ns.NetswitchSensorPort)
	} else {
		return ns.NetswitchUrlOff
	}
}

func (ns *NetSwitch) String() string {
	return fmt.Sprintf("(NetSwitch MachineId=%v On=%v)",
		ns.Id, ns.On)
}

func (ns *NetSwitch) ApplyConfig(updates chan<- string, xmppClient *xmpp.Xmpp, userId int64) (err error) {
	ns.muChInit.Lock()
	if ns.chSingle == nil {
		log.Printf("make(chan int, 1)")
		ns.chSingle = make(chan int, 1)
		ns.chSingle <- 1
	} else {
		log.Printf("ns.chSingle != nil")
	}
	ns.muChInit.Unlock()
	select {
	case <-ns.chSingle:
		log.Printf("not running")
		break
	default:
		log.Println("apply config already running")
		return fmt.Errorf("apply config already running")
	}
	cfg := mfi.Config{
		Host: ns.NetswitchHost,
	}
	statusMsg := xmpp.Message{
		Remote: global.ServerJabberId,
		Data: xmpp.Data{
			Command:    commands.GATEWAY_APPLIED_CONFIG_1,
			LocationId: global.Cfg.Main.LocationId,
			MachineId:  ns.Id,
			UserId:     userId,
		},
	}
	if err := cfg.RunStep1Wifi(); err != nil {
		ns.chSingle <- 1
		statusMsg.Data.Error = true
		statusMsg.Data.ErrorMessage = fmt.Sprintf("step 1 wifi: %v", err)
		if err := xmppClient.Send(statusMsg); err != nil {
			log.Printf("xmpp command send: %v", err)
		}
		return fmt.Errorf(statusMsg.Data.ErrorMessage)
	}
	if err = xmppClient.Send(statusMsg); err != nil {
		log.Printf("xmpp command send: %v", err)
	}

	go func() {
		statusMsg := xmpp.Message{
			Remote: global.ServerJabberId,
			Data: xmpp.Data{
				Command:    commands.GATEWAY_APPLIED_CONFIG_2,
				LocationId: global.Cfg.Main.LocationId,
				MachineId:  ns.Id,
				UserId:     userId,
			},
		}
		if err := cfg.RunStep2PushConfig(); err != nil {
			statusMsg.Data.Error = true
			statusMsg.Data.ErrorMessage = err.Error()
			updates <- err.Error()
		}
		if err = xmppClient.Send(statusMsg); err != nil {
			log.Printf("xmpp command send: %v", err)
		}
		ns.chSingle <- 1
	}()
	return
}

func (ns *NetSwitch) FetchNetswitchStatus() (
	relayState string,
	current float64,
	err error,
) {
	urlStatus := ns.UrlOn()
	log.Printf("urlStatus = %v", urlStatus)
	if urlStatus == "" {
		return "", 0, fmt.Errorf("NetSwitch status url for Machine %v empty", ns.Id)
	}
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(urlStatus)
	if err != nil {
		return "", 0, fmt.Errorf("http get url status: %v", err)
	}
	defer resp.Body.Close()
	mfi := MfiSwitch{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&mfi); err != nil {
		return "", 0, fmt.Errorf("json decode:", err)
	}
	if len(mfi.Sensors) < 1 {
		return "", 0, fmt.Errorf("0 sensors in list")
	}
	sensor := mfi.Sensors[0]
	if sensor.Relay == 0 {
		relayState = commands.NETSWITCH_RELAY_OFF
	} else if sensor.Relay == 1 {
		relayState = commands.NETSWITCH_RELAY_ON
	}
	current = sensor.Current
	return
}

//{"sensors":[{"output":1,"power":0.0,"energy":0.0,"enabled":0,"current":0.0,"voltage":233.546874046,"powerfactor":0.0,"relay":1,"lock":0}],"status":"success"}

type MfiSwitch struct {
	Sensors []MfiSensor `json:"sensors"`
	Status  string      `json:"status"`
}

func (swi *MfiSwitch) On() bool {
	relay := swi.Sensors[0].Relay
	switch relay {
	case 0:
		return false
		break
	case 1:
		return true
		break
	}
	log.Fatalf("unknown relay status %v, terminating", relay)
	return false
}

type MfiSensor struct {
	Output      int     `json:"output"`
	Power       float64 `json:"power"`
	Energy      float64 `json:"energy"`
	Enabled     float64 `json:"enabled"`
	Current     float64 `json:"current"`
	Voltage     float64 `json:"voltage"`
	PowerFactor float64 `json:"powerfactor"`
	Relay       int     `json:"relay"`
	Lock        int     `json:"lock"`
}
