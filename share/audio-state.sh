#!/bin/bash

IFS=$'\n'
line=($(amixer get Headset))
IFS=':'
Left=(${line[5]})
Rigt=(${line[6]})

FIELDS='%-20s  %-20s\n'
printf "$FIELDS" ${Left[0]#  }         ${Rigt[0]#  }
printf "$FIELDS" ${Left[1]# Playback } ${Rigt[1]# Playback }
