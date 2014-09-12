
// info-anvl provides a browser UI for anvl config & docs
//   https://github.com/networkimprov/info-anvl
//
// "info.go" HTTP server app
//
// Copyright 2014 by Liam Breck


package main

import (
  "os"
  "path/filepath"
  "io"
  "io/ioutil"
  "fmt"
  "net/http"
  "text/template"
  "os/exec"
  "strings"
  "sync"
  "github.com/gorilla/websocket"
  "github.com/kr/pty"
)

var sDirname = filepath.Dir(os.Args[0])

var sTmpl *template.Template
type tPageData struct { Title string; Main []byte }

func main() {
    var err error
    sTmpl, err = template.ParseFiles(sDirname+"/pagetmpl.html")
    if err != nil { panic(err) }
    fmt.Println("ready")
    http.HandleFunc("/", reqDoc)
    http.HandleFunc("/stat", reqStat)
    http.HandleFunc("/con", reqCon)
    http.HandleFunc("/ws", reqWs)
    http.HandleFunc("/term.js", reqFile)
    http.ListenAndServe(":80", nil)

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

type tCommand struct { name, c string; buf []byte }
var sCmdList = [...]tCommand {
  { name:"Date",       c:"/bin/date" },
  { name:"Battery",    c:"/bin/sh "+sDirname+"/batt-state.sh /sys/class/power_supply/bq24190-battery" },
  { name:"Speaker",    c:"/usr/bin/printf " },
  { name:"LEDs",       c:"/bin/sh "+sDirname+"/led-state.sh /sys/class/leds" },
  { name:"CPU",        c:"/bin/bash "+sDirname+"/cpu-state.sh" },
  { name:"RAM",        c:"/bin/sh -c top\t-bn1\t-p1|sed\t-n\t4,5p" },
  { name:"Disk",       c:"/bin/df -m /" },
  { name:"WLANs",      c:"/usr/bin/printf " },
  { name:"Wifi",       c:"/sbin/ip addr show mlan0" },
  { name:"Wifi P2P",   c:"/sbin/ip addr show p2p0" },
  { name:"USB",        c:"/sbin/ip addr show usb1" },
  { name:"Kernel",     c:"/bin/uname -srv" },
  { name:"Processes",  c:"/bin/ps -FN --pid 1,2 --ppid 2,"+fmt.Sprintf("%d", os.Getpid())+" -C agetty --sort=-rss" },
}

var sStatDoor sync.Mutex

func reqStat(oResp http.ResponseWriter, iReq *http.Request) {
  sStatDoor.Lock()
  var aPidLink = make(chan int, len(sCmdList))
  var aHerd sync.WaitGroup
  var fExec func(*tCommand)
  var fRun = func() {
    var aProc *tCommand
    for a := range sCmdList {
      if sCmdList[a].name == "Processes" {
        aProc = &sCmdList[a]
      } else {
        aHerd.Add(1)
        go fExec(&sCmdList[a])
      }
    }
    aChildren := "--ppid "
    for a := 1; a < len(sCmdList); a++ {
      aChildren += fmt.Sprintf("%d,", <- aPidLink)
    }
    aTmp := aProc.c
    aProc.c = strings.Replace(aProc.c, "--ppid ", aChildren, 1)
    fExec(aProc)
    aProc.c = aTmp
    aHerd.Wait()
  }
  fExec = func(aC *tCommand) {
fmt.Println(aC.name, aC.c)
    aArgs := strings.Split(aC.c, " ")
    aCmd := exec.Command(aArgs[0], aArgs[1:]...)
    aPipe, err := aCmd.StdoutPipe()
    if err != nil { panic(err) }
    err = aCmd.Start()
    if err != nil { panic(err) }
    if aC.name != "Processes" {
      aPidLink <- aCmd.Process.Pid
    }
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
    if aC.name != "Processes" {
      aHerd.Done()
    }
  }

  fRun()
  aTable := make([]byte, 0, 8192)
  aTable = append(aTable, "<table>\n"...);
  const kRow = "<tr><td>%s</td><td><pre>%s</pre></td></tr>\n"
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


