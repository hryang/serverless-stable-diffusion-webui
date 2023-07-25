#! /bin/bash

for d in agent proxy; do
  (go build -o sd$d ../../cmd/$d)
done

pkill sdagent
pkill sdproxy

agent_ports=()
agent_ports+=("1235")
agent_ports+=("1236")
sd_services=()
#sd_services+=("http://47.96.113.137:7860/")
sd_services+=("http://sd.fc-stable-diffusion.1050834996213541.cn-hangzhou.fc.devsapp.net/")
sd_services+=("http://sd.fc-stable-diffusion-api.1050834996213541.cn-hangzhou.fc.devsapp.net/")

sqlite3 test.db "CREATE TABLE IF NOT EXISTS stable_diffusion_services (SERVICE_NAME text primary key not null, SERVICE_ENDPOINT text)"

end=$((${#sd_services[@]}-1))
for i in $(seq 0 $end); do
    echo "create agent on port ${agent_ports[i]} for service ${sd_services[i]}"
    ./sdagent -port=${agent_ports[i]} -sqlite-file=./test.db -target=${sd_services[i]} > sdagent_${i}.log &
    sqlite3 test.db "INSERT OR REPLACE INTO stable_diffusion_services (SERVICE_NAME, SERVICE_ENDPOINT) VALUES ('s${i}', 'http://127.0.0.1:${agent_ports[i]}');"
done

echo "create proxy ..."
./sdproxy -port=1234 -sqlite-file=./test.db  > sdproxy.log &
