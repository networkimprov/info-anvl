
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

var sDirname = filepath.Dir(os.Args[0])+"/share"

var sTmpl *template.Template
type tPageData struct { Title string; Main []byte }

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
  { name:"Kernel",     c:"/bin/uname -srv" },
  { name:"Battery",    c:"/bin/bash "+sDirname+"/batt-state.sh" },
  { name:"Disk",       c:"/bin/df -m /" },
  { name:"CPU",        c:"/bin/bash "+sDirname+"/cpu-state.sh" },
  { name:"RAM",        c:"/bin/bash "+sDirname+"/mem-state.sh" },
  { name:"WLANs",      c:"/bin/bash "+sDirname+"/wlan-state.sh" },
  { name:"Wifi",       c:"/bin/ip addr show mlan0" },
  { name:"P2P",        c:"/bin/ip addr show p2p0" },
  { name:"USB",        c:"/bin/ip addr show usb0" },
  { name:"Speaker",    c:"/bin/printf " },
  { name:"LEDs",       c:"/bin/bash "+sDirname+"/led-state.sh" },
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
    } else {
      aHerd.Add(1)
      go fExec(&sCmdList[a])
    }
  }
  aHerd.Wait()
  fExec(aProc)

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


