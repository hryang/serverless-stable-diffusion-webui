package main

import (
	"flag"
	"fmt"

	"github.com/hryang/stable-diffusion-webui-proxy/pkg/datastore"
	"github.com/hryang/stable-diffusion-webui-proxy/pkg/proxy"
)

func main() {
	target := flag.String("target", "", "the downstream service endpoint")
	port := flag.Int("port", 0, "the agent port number")
	sqliteFile := flag.String("sqlite-file", "", "the sqlite file")

	flag.Parse()

	if *target == "" {
		panic("invalid target")
	}
	if *port == 0 {
		panic("invalid port")
	}
	if *sqliteFile == "" {
		panic("invalid datastore")
	}

	fmt.Printf("target: %s, port: %d, sqlite file: %s\n", *target, *port, *sqliteFile)
	// TODO: Make dbType configurable.
	s := proxy.NewServer(*target, datastore.SQLite, *sqliteFile)

	s.Echo.Logger.Fatal(s.Start(fmt.Sprintf("0.0.0.0:%d", *port)))
}
