#!/bin/sh

FIELDS='%-15s%-11s%-10s%-10s%s\n'

cd /sys/class/leds
printf $FIELDS Device Brightness Delay_on Delay_off Trigger

for LED in *; do
  exec < $LED/brightness; read brightness
  exec < $LED/trigger; read trigger
  trigger="${trigger#*\[}"
  trigger="${trigger%\]*}"
  if [ "$trigger" = timer ]; then
    exec < $LED/delay_on ; read delay_on
    exec < $LED/delay_off; read delay_off
  else
    delay_on='--'
    delay_off='--'
  fi
  printf $FIELDS "$LED" "$brightness" "$delay_on" "$delay_off" "$trigger"
done

