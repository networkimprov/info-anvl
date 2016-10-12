#! /bin/bash


for N in A B; do
  exec < /proc/stat
  read line
  set $line
  shift # remove cpu label
  let total$N=0
  let a=1
  while let "a < 8"; do
    let total$N+=$1
    let v$N$a=$1
    let a+=1
    shift
  done
  if [ $N = A ]; then
    sleep 0.25
  fi
done

while let "a > 1"; do
  let a-=1
  #let "lhs = vB$a - vA$a"; echo -n "$lhs "
  let "vA$a = ( vB$a - vA$a ) * 1000 / ( totalB - totalA )"
  let "lhs = vA$a / 10"
  let "rhs = vA$a % 10"
  args="$lhs $rhs $args"
done
#let "lhs = totalB - totalA"; echo "= $lhs"

printf "User %%  Niced   System  Idle    IOWait  IRQ     SoftIRQ\n"
printf "%4d.%d  %4d.%d  %4d.%d  %4d.%d  %4d.%d  %4d.%d  %5d.%d\n" $args

