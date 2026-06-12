// z2r-sim is a local simulator of the fiskaly SIGN IT TEST API with fault
// injection — chaos engineering for fiscal compliance. Point any client
// (including z2r-mcp --base-url) at it.
package main

import (
	"flag"
	"log"
	"net/http"

	"z2r/internal/sim"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8484", "listen address")
	scenario := flag.String("scenario", "happy", "happy | ade-outage | slow-ade")
	flag.Parse()

	server := sim.New(sim.Scenario(*scenario))
	log.Printf("SIGN IT simulator on http://%s (scenario: %s)", *addr, *scenario)
	log.Fatal(http.ListenAndServe(*addr, server.Handler()))
}
