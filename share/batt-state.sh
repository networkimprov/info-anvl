#!/bin/sh

FIELDS='%-7s%-15s%s\n'

exec < $1/online; read online
exec < $1/status; read status
exec < $1/health; read health

printf $FIELDS Online Status Health
printf $FIELDS "$online" "$status" "$health"

