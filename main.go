package main

import (
	"flag"
	"fmt"

	"github.com/jvermillard/moxy/server"
)

var listen string
var srv string
var debug bool
var trace bool

func setupFlags() {
	flag.StringVar(&listen, "listen", "0.0.0.0:1883", "the MQTT address and port to listen")
	flag.StringVar(&srv, "server", "iot.eclipse.org:1883", "the target MQTT server to proxify")
	flag.BoolVar(&debug, "v", false, "dumps verbose debug information")
	flag.BoolVar(&trace, "t", false, "trace every communication to JSON files")
	flag.Parse()
}

func main() {
	fmt.Println("Moxy: a MQTT proxy")
	setupFlags()
	if debug {
		fmt.Println("verbose mode enabled")
	}
	server.StartServer(listen, srv, debug, trace)
}
