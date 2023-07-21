#! /bin/bash

for d in agent proxy; do
  (go build -o sd$d ./$d)
done

pkill sdagent
pkill sdproxy

./sdagent > sdagent.log & 
./sdproxy > sdproxy.log &
