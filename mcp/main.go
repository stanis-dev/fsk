// Command fiskaly-mcp is the fiskaly MCP server. It is intentionally empty: it
// registers no tools and exists only to establish the wiring into a consumer
// agent. Real tools (docs grounding, action tools, sandbox controls) are added
// later; this is the baseline for the with-MCP arm of the eval differential.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "fiskaly", Version: "v0.0.0"}, nil)
	// No tools registered yet.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
