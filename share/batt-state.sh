#!/bin/sh

FIELDS='%-9s%-9s%-15s%s\n'

cd /sys/class/power_supply
exec < bq24190-battery/online ; read online
exec < bq24190-battery/health ; read health
exec < bq27425-0/capacity     ; read charge
exec < bq27425-0/current_now  ; read status

if [ "$status" -lt 0 ]; then
  status=Discharging
else
  status=Charging
fi

printf $FIELDS Online Charge Status Health
printf $FIELDS "$online" "$charge" "$status" "$health"

