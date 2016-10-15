
// info-anvl provides a browser UI for anvl config & docs
//   https://github.com/networkimprov/info-anvl
//
// "info.go" HTTP server app
//
// Copyright 2014 by Liam Breck


package main

// #cgo LDFLAGS: -lasound
// #include <stdlib.h>
// #include "alsactl.h"
import "C"

import (
  "bytes"
  "os"
  "path/filepath"
  "io"
  "io/ioutil"
  "fmt"
  "net"
  "net/http"
  "text/template"
  "os/exec"
  "regexp"
  "strconv"
  "strings"
  "sync"
  "time"
  "unsafe"
  "github.com/gorilla/websocket"
  "github.com/kr/pty"
)

var sDirname = filepath.Dir(os.Args[0])+"/share"

var sTmpl *template.Template
type tPageData struct { Title string; Main []byte }

const kSocket = "/tmp/info-anvl-wpa-socket"
var sConn *net.UnixConn

func main() {
    var err error
    if len(os.Args) == 2 {
        sDirname = os.Args[1]
        var aInfo os.FileInfo
        aInfo, err = os.Stat(sDirname)
        if err != nil || !aInfo.IsDir() {
          fmt.Fprintln(os.Stderr, sDirname, "is not a directory")
          os.Exit(1)
        }
    }
    for i,a := range sCmdList { sCmdList[i].c = strings.Replace(a.c, "###", sDirname, 1) }
    sTmpl, err = template.ParseFiles(sDirname+"/pagetmpl.html")
    if err != nil { panic(err) }

    os.Remove(kSocket)
    aHost, err := net.ResolveUnixAddr("unixgram", "/run/wpa_supplicant/mlan0")
    if err != nil { panic(err) }
    aPeer, err := net.ResolveUnixAddr("unixgram", kSocket)
    if err != nil { panic(err) }
    sConn, err = net.DialUnix("unixgram", aPeer, aHost)
    if err != nil { panic(err) }

    fmt.Println("ready")
    http.HandleFunc("/", reqDoc)
    http.HandleFunc("/stat", reqStat)
    http.HandleFunc("/con", reqCon)
    http.HandleFunc("/ws", reqWs)
    http.HandleFunc("/term.js", reqFile)
    http.ListenAndServe(":80", nil)

    sConn.Close()
}

func reqFile(oResp http.ResponseWriter, iReq *http.Request) {
  aF, err := os.Open(sDirname+iReq.URL.Path)
  if err != nil { panic(err) }
  _, err = io.Copy(oResp, aF)
  if err != nil { panic(err) }
}

func reqDoc(oResp http.ResponseWriter, iReq *http.Request) {
    aBuf, err := ioutil.ReadFile(sDirname+"/doc.html")
    if err != nil { panic(err) }
    sTmpl.Execute(oResp, tPageData{ Title:"ANVL Docs", Main:aBuf })
}

type tCommand struct { name, c string; f func(*tCommand); buf []byte }

func statDate(oC *tCommand) {
    aS := time.Now().Format("15:04:05 MST Mon 02 Jan 2006")
    oC.buf = append(oC.buf, aS...)
    fmt.Println(oC.name)
}

func statKernel(oC *tCommand) {
    oC.buf,_ = ioutil.ReadFile("/proc/version")
    var aBlank []byte
    oC.buf = bytes.Replace(oC.buf, []byte("version "), aBlank, 2)
    oC.buf = bytes.Replace(oC.buf, []byte("(liam@localhost) "), aBlank, 1)
    fmt.Println(oC.name)
}

func statBattery(oC *tCommand) {
  const kCharger = "/sys/class/power_supply/bq24190-battery/"
  const kGauge = "/sys/class/power_supply/bq27425-0/"
  aOnline  ,_ := ioutil.ReadFile(kCharger+"online")
  aHealth  ,_ := ioutil.ReadFile(kCharger+"health")
  aCharge  ,_ := ioutil.ReadFile(kGauge+"capacity")
  aCurrent ,_ := ioutil.ReadFile(kGauge+"current_now")

  aStatus := "Charging"
  if a,_ := strconv.Atoi(string(aCurrent)); a < 0 {
    aStatus = "Discharging"
  }
  aCharge[len(aCharge)-1] = '%'
  aOnline = aOnline[:len(aOnline)-1]
  aHealth = aHealth[:len(aHealth)-1]

  const kFields = "%-11s  %-11s  %-11s  %s\n"
  oC.buf = []byte(fmt.Sprintf(kFields+kFields,
    "Online", "Charge", "Status", "Health", aOnline, aCharge, aStatus, aHealth))
  fmt.Println(oC.name)
}

func statCpu(oC *tCommand) {
  var aPass [2][7]int
  var aTotl = [2]int{0,0}
  for a:=0; a < len(aPass); a++ {
    aBuf,_ := ioutil.ReadFile("/proc/stat")
    aLine := bytes.SplitN(aBuf, []byte{'\n'}, 3)
    aLine = bytes.SplitN(aLine[1], []byte{' '}, 9)
    for aN:=0; aN < len(aPass[0]); aN++ {
      aPass[a][aN],_ = strconv.Atoi(string(aLine[aN+1]))
      aTotl[a] += aPass[a][aN]
    }
    if a == 0 {
      time.Sleep(time.Duration(150)*time.Millisecond)
    }
  }
  var aArg string
  for aN:=0; aN < len(aPass[0]); aN++ {
    aPass[0][aN] = (aPass[1][aN] - aPass[0][aN]) * 1000 / (aTotl[1] - aTotl[0])
    aArg += fmt.Sprintf("%4d.%d  ", aPass[0][aN] / 10, aPass[0][aN] % 10)
  }
  oC.buf = []byte(fmt.Sprintf("%-6s  %-6s  %-6s  %-6s  %-6s  %-6s  %-6s\n%s\n",
    "User %", "Niced", "System", "Idle", "IOWait", "IRQ", "SoftIRQ", aArg))
  fmt.Println(oC.name)
}

func InsertByte(s []byte, p int, b byte) []byte {
  s = s[:len(s)+1]
  copy(s[p+1:], s[p:])
  s[p] = b
  return s
}

var sRamKb = regexp.MustCompile(" *([0-9]+)")

func statRam(oC *tCommand) {
  aBuf,_ := ioutil.ReadFile("/proc/meminfo")
  aLine := bytes.SplitN(aBuf, []byte{'\n'}, 6)
  var aPair [5][][]byte
  for a := 0; a < 5; a++ {
    aPair[a] = bytes.SplitN(aLine[a], []byte{':'}, 2)
    aPair[a][1] = sRamKb.FindSubmatch(aPair[a][1])[1]
    if len(aPair[a][1]) > 3 {
      aPair[a][1] = InsertByte(aPair[a][1], len(aPair[a][1])-3, ',')
    }
  }
  const kFieldTxt = "%-10skB   %-12s   %-12s   %-12s   %-12s\n"
  const kFieldNum = "%12s   %12s   %12s   %12s   %12s\n"
  oC.buf = []byte(fmt.Sprintf(kFieldTxt+kFieldNum,
    aPair[0][0], aPair[1][0], aPair[2][0], aPair[3][0], aPair[4][0],
    aPair[0][1], aPair[1][1], aPair[2][1], aPair[3][1], aPair[4][1]))
  fmt.Println(oC.name)
}

func statWlans(oC *tCommand) {
  _,err := sConn.Write([]byte("STATUS"))
  if err != nil { panic(err) }
  aBuf := make([]byte, 2048)
  aLen, err := sConn.Read(aBuf)
  if err != nil { panic(err) }
  var aSsid string
  aLine := bytes.Split(aBuf[:aLen], []byte{'\n'})
  for aN:=0; aN < len(aLine); aN++ {
    aPair := bytes.SplitN(aLine[aN], []byte{'='}, 2)
    if string(aPair[0]) == "ssid" {
      aSsid = string(aPair[1])
      oC.buf = []byte("<strong style=\"color:blue\">"+aSsid+"</strong>  ")
      break
    }
  }
  aList,_ := ioutil.ReadDir("/etc/netctl")
  for _,a := range aList {
    if aLan := a.Name(); len(aLan) > 6 && aLan[:6] == "mlan0-" {
      if aLan = aLan[6:]; aLan != aSsid {
        oC.buf = append(oC.buf, aLan+"  "...)
      }
    }
  }
  oC.buf = append(oC.buf, '\n')
  fmt.Println(oC.name)
}

func statAudio(oC *tCommand) {
  var aCtx unsafe.Pointer = C.alsactl_open()

  oC.buf = []byte("<table style=\"border-spacing:0\"><tr>\n")
  aList := []string{"DAC1 Analog", "DAC1 Digital Fine", "DAC1 Digital Coarse"}
  for _,a := range aList {
    var aMin, aMax, aVol C.long
    var aOk C.int = C.alsactl_get_volume(aCtx, C.CString(a), &aMin, &aMax, &aVol)
    if int(aOk) != 0 { continue }
    const kFields = "<td>%-13s\n  vol   range\n  %3d   %d-%d\n</td><td>    </td>\n"
    oC.buf = append(oC.buf, fmt.Sprintf(kFields, a, int(aVol), int(aMin), int(aMax))...)
  }
  oC.buf = append(oC.buf, "</tr><tr><td> </td></tr><tr>\n"...)
  const kFields = "<td>%-13s\n%13v\n</td><td>    </td>\n"
  aList = []string{"DAC1 Analog", "HandsfreeL"}
  for _,a := range aList {
    var aVal C.int
    var aOk C.int = C.alsactl_get_switch(aCtx, C.CString(a), &aVal)
    if int(aOk) != 0 { continue }
    aStr := "on"; if int(aVal) == 0 { aStr = "off" }
    oC.buf = append(oC.buf, fmt.Sprintf(kFields, a, aStr)...)
  }
  aList = []string{"HandsfreeL Mux"}
  for _,a := range aList {
    var aStr *C.char
    var aOk C.int = C.alsactl_get_enum(aCtx, C.CString(a), &aStr)
    if int(aOk) != 0 { continue }
    oC.buf = append(oC.buf, fmt.Sprintf(kFields, a, C.GoString(aStr))...)
    C.free(unsafe.Pointer(aStr))
  }
  oC.buf = append(oC.buf, "</tr></table>"...)

  C.alsactl_close(aCtx)
  fmt.Println(oC.name)
}

func statLeds(oC *tCommand) {
  const kLeds = "/sys/class/leds/"
  aList,_ := ioutil.ReadDir(kLeds)
  const kFields = "%-15s%-11s%-10s%-10s%s\n"

lRestart:
  oC.buf = []byte(fmt.Sprintf(kFields, "Device", "Brightness", "Delay_on", "Delay_off", "Trigger"))

  for _,a := range aList {
    aBright,_ := ioutil.ReadFile(kLeds+a.Name()+"/brightness")
    aTriggr,_ := ioutil.ReadFile(kLeds+a.Name()+"/trigger")
    aBright = aBright[:len(aBright)-1]
    aTriggr = aTriggr[ bytes.IndexRune(aTriggr, '[')+1 : bytes.IndexRune(aTriggr, ']') ]
    var aDelayOn, aDelayOff []byte
    if bytes.Equal(aTriggr, []byte("timer")) {
      aDelayOn ,_ = ioutil.ReadFile(kLeds+a.Name()+"/delay_on")
      aDelayOff,_ = ioutil.ReadFile(kLeds+a.Name()+"/delay_off")
      if len(aDelayOn) == 0 || len(aDelayOff) == 0 {
        goto lRestart
      }
      aDelayOn  = aDelayOn [:len(aDelayOn )-1]
      aDelayOff = aDelayOff[:len(aDelayOff)-1]
    } else {
      aDelayOn = []byte("--")
      aDelayOff = aDelayOn
    }
    oC.buf = append(oC.buf, fmt.Sprintf(kFields, a.Name(), aBright, aDelayOn, aDelayOff, aTriggr)...)
  }
  fmt.Println(oC.name)
}

var sCmdList = [...]tCommand {
  { name:"Date",       f:statDate },
  { name:"Kernel",     f:statKernel },
  { name:"Battery",    f:statBattery },
  { name:"Disk",       c:"/bin/df -m /" },
  { name:"CPU",        f:statCpu },
  { name:"RAM",        f:statRam },
  { name:"WLANs",      f:statWlans },
  { name:"Wifi",       c:"/bin/ip addr show mlan0" },
  { name:"P2P",        c:"/bin/ip addr show p2p0" },
  { name:"USB",        c:"/bin/ip addr show usb0" },
  { name:"Audio",      f:statAudio },
  { name:"LEDs",       f:statLeds },
  { name:"PS",         c:"/bin/ps -FN --pid 2 --ppid 2,"+fmt.Sprintf("%d", os.Getpid())+" --sort=-time,-rss" },
}

var sStatDoor sync.Mutex

func reqStat(oResp http.ResponseWriter, iReq *http.Request) {
  sStatDoor.Lock()
  var aHerd sync.WaitGroup

  fExec := func(aC *tCommand) {
    aArgs := strings.Split(aC.c, " ")
    aCmd := exec.Command(aArgs[0], aArgs[1:]...)
    aPipe, err := aCmd.StdoutPipe()
    if err != nil { panic(err) }
    err = aCmd.Start()
    if err != nil { panic(err) }
    aBuf := make([]byte, 512)
    for err = nil; err == nil; {
      var aLen int
      aLen, err = aPipe.Read(aBuf)
      aC.buf = append(aC.buf, aBuf[:aLen]...)
    }
    if err != io.EOF {
      aC.buf = append(aC.buf, fmt.Sprintf("\n%s\n", err)...)
    }
    if len(aC.buf) > 0 {
      aC.buf = aC.buf[:len(aC.buf)-1]
    }
    err = aCmd.Wait()
    if err != nil { fmt.Fprintln(os.Stderr, aC.name, err) }
fmt.Println(aC.name, aC.c)
    if aC.name != "PS" {
      aHerd.Done()
    }
  }

  var aProc *tCommand
  for a := range sCmdList {
    if sCmdList[a].name == "PS" {
      aProc = &sCmdList[a]
    } else if len(sCmdList[a].c) > 0 {
      aHerd.Add(1)
      go fExec(&sCmdList[a])
    } else {
      go sCmdList[a].f(&sCmdList[a])
    }
  }
  fExec(aProc)
  aHerd.Wait()

  aTable := make([]byte, 0, 8192)
  aTable = append(aTable, "<table>\n"...);
  const kRow = "<tr><td class=\"stat\">%s</td><td><pre>%s</pre></td></tr>\n"
  for a := 0; a < len(sCmdList); a++ {
    aTable = append(aTable, fmt.Sprintf(kRow, sCmdList[a].name, sCmdList[a].buf)...) //. protect from html
    sCmdList[a].buf = []byte{}
  }
  aTable = append(aTable, "</table>"...);
  sTmpl.Execute(oResp, tPageData{ Title:"ANVL System Stats", Main:aTable })
  sStatDoor.Unlock()
}

func reqCon(oResp http.ResponseWriter, iReq *http.Request) {
  aBuf, err := ioutil.ReadFile(sDirname+"/console.html")
  if err != nil { panic(err) }
  sTmpl.Execute(oResp, tPageData{ Title:"ANVL Console", Main:aBuf })
    
}

var sWsInit = websocket.Upgrader {
  ReadBufferSize:  1024,
  WriteBufferSize: 1024,
}

func reqWs(oResp http.ResponseWriter, iReq *http.Request) {
  aSock, err := sWsInit.Upgrade(oResp, iReq, nil)
  if err != nil { panic(err) }
  aF, err := pty.Start(exec.Command("bash"))
  if err != nil { panic(err) }
  go func() {
    for {
      _, aIn, err := aSock.ReadMessage()
      if err != nil {
        aF.Close()
        return
      }
      aLen, err := aF.Write(aIn)
      if err != nil {
        aSock.Close()
        return
      }
      if aLen < len(aIn) { panic("pty write overflow") }
    }
  }()
  aOut := make([]byte, sWsInit.WriteBufferSize)
  for {
    aLen, err := aF.Read(aOut)
    if err != nil {
      aSock.Close()
      return
    }
    err = aSock.WriteMessage(websocket.TextMessage, aOut[:aLen])
    if err != nil {
      aF.Close()
      return
    }
  }
}


