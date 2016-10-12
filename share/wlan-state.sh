#!/bin/sh

stats=($(/bin/wpa_cli -i mlan0 status))

for var in ${stats[@]}; do
  if [ "${var%%=*}" = ssid ]; then
    ssid=${var#ssid=}
    printf "<strong style=\"color:blue\">$ssid</strong>  "
    break
  fi
done

cd /etc/netctl
list=(mlan0-*)
for var in ${list[@]#mlan0-}; do
  if [ "$var" != "$ssid" ]; then printf "$var  "; fi
done

printf '\n'
