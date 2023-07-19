package main

import (
	"github.com/hryang/stable-diffusion-webui-proxy/pkg/proxy"
)

func main() {
	s := proxy.NewAgent(
		"http://sd.fc-stable-diffusion.1050834996213541.cn-hangzhou.fc.devsapp.net/")

	s.Echo.Logger.Fatal(s.Start("0.0.0.0:1234"))
}
