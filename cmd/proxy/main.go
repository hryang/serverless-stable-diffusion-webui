package main

import (
	"github.com/hryang/stable-diffusion-webui-proxy/pkg/proxy"
)

func main() {
	s := proxy.NewServer(
		"http://127.0.0.1:1235")

	s.Echo.Logger.Fatal(s.Start("0.0.0.0:1234"))
}
