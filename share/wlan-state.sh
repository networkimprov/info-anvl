#!/bin/sh

cd /etc/netctl
list=(mlan0-*)
echo ${list[@]#mlan0-}
