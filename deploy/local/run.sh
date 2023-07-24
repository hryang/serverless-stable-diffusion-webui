#! /bin/bash

for d in agent proxy; do
  (go build -o sd$d ./$d)
done

pkill sdagent
pkill sdproxy

./sdagent -port=1235 -sqlite-file=./test.db -target=http://sd.fc-stable-diffusion.1050834996213541.cn-hangzhou.fc.devsapp.net/ > sdagent.log & 
./sdproxy -port=1234 -sqlite-file=./test.db -target=http://127.0.0.1:1235 > sdproxy.log &
