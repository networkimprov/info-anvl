#!/bin/bash

FIELDTXT="%-10skB   %-12s   %-12s   %-12s   %-12s\n"
FIELDNUM="%'12d   %'12d   %'12d   %'12d   %'12d\n"

exec < /proc/meminfo
read -ra a1
read -ra a2
read -ra a3
read -ra a4
read -ra a5

export LC_NUMERIC=en_US
printf "$FIELDTXT" ${a1[0]%:} ${a2[0]%:} ${a3[0]%:} ${a4[0]%:} ${a5[0]%:}
printf "$FIELDNUM" ${a1[1]}   ${a2[1]}   ${a3[1]}   ${a4[1]}   ${a5[1]}
