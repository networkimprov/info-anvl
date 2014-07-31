
// info-anvl provides a browser UI for anvl config & docs
//   https://github.com/networkimprov/info-anvl
//
// "info.js" node.js HTTP server app
//
// Copyright 2014 by Liam Breck


var lHttp = require('http');
var lChild = require('child_process');
var lFs = require('fs');

var sPort = 80;
var sTmpl = lFs.readFileSync(__dirname+'/pagetmpl.html').toString();
var sCmdList = [
  { n:'Date',       b:null, c:'/bin/date' },
  { n:'Battery',    b:null, c:'/bin/sh '+__dirname+'/batt-state.sh /sys/class/power_supply/bq24190-battery' },
  { n:'Speaker',    b:null, c:'/usr/bin/printf ' },
  { n:'LEDs',       b:null, c:'/bin/sh '+__dirname+'/led-state.sh /sys/class/leds' },
  { n:'CPU',        b:null, c:'/bin/bash '+__dirname+'/cpu-state.sh' },
  { n:'RAM',        b:null, c:'/bin/sh -c top\t-bn1\t-p1|sed\t-n\t4,5p' },
  { n:'Disk',       b:null, c:'/bin/df -m /' },
  { n:'WLANs',      b:null, c:'/usr/bin/printf ' },
  { n:'Wifi',       b:null, c:'/sbin/ip addr show mlan0' },
  { n:'Wifi P2P',   b:null, c:'/sbin/ip addr show p2p0' },
  { n:'USB',        b:null, c:'/sbin/ip addr show usb1' },
  { n:'Kernel',     b:null, c:'/bin/uname -srv' },
  { n:'Processes',  b:null, c:'/bin/ps -FN --pid 1,2 --ppid 2,'+process.pid+' -C agetty --sort=-rss' }
];

var sSrvr = lHttp.createServer(handleRequest);
sSrvr.listen(sPort, function(err) {
  if (err) throw err;
  console.log("ready");
});

function handleRequest(iReq, oResponse) {
  switch (iReq.url) {
  case '/':     reqDoc (fRespond); break;
  case '/stat': reqStat(fRespond); break;
  case '/con':  reqCon (fRespond); break;
  default:      fRespond(null, 'error', 'page not found', 400);
  }
  function fRespond(err, title, body, code) {
    if (err) throw err;
    oResponse.writeHead(code ? code : 200, {'Content-Type': 'text/html'});
    oResponse.end(sTmpl.replace('untitled', title).replace('unwritten', body));
  }
}

function reqDoc(iCallback) {
  var aBuf = lFs.readFileSync(__dirname+'/doc.html');
  iCallback(null, 'ANVL Docs', aBuf.toString());
}

function reqCon(iCallback) {
  iCallback(null, 'ANVL Console', 'dials n knobs here');
}

function reqStat(iCallback) {
  var aChildren = '--ppid ';
  for (var aN=0; aN < sCmdList.length; ++aN)
    fExec(sCmdList[aN]);
  function fExec(cmd) {
    var aArgs = (cmd.n === 'Processes' ? cmd.c.replace('--ppid ', aChildren) : cmd.c).split(' ');
    var aOp = aArgs.shift();
    var aC = lChild.execFile(aOp, aArgs, fDone);
    aChildren += aC.pid+',';
    function fDone(err, stdout, stderr) {
      if (stderr.length) console.log(stderr.toString());
      cmd.b = stdout.slice(0, -1);
      if (--aN > 0)
        return;
      var aTable = '<table>\n';
      for (aN=0; aN < sCmdList.length; ++aN)
        aTable += '<tr><td>'+sCmdList[aN].n+'</td><td><pre>'+sCmdList[aN].b.toString()+'</pre></td></tr>\n';
      aTable += '</table>';
      iCallback(null, 'ANVL System Stats', aTable);
    }
  }
}

