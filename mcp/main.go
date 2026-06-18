// Command fiskaly-mcp is the fiskaly MCP server. It serves the curated SIGN IT
// documentation corpus through two read-only tools, search_fiskaly_docs and
// fetch_fiskaly_doc, so a consumer agent can ground its integration in the docs.
package main

import (
	"context"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"fiskaly-mcp/corpus"
	"fiskaly-mcp/telemetry"
)

func main() {
	c, err := corpus.Load()
	if err != nil {
		log.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fiskaly", Version: "v0.1.0"}, nil)
	registerTools(server, c)

	if path := os.Getenv("FISKALY_MCP_TELEMETRY"); path != "" {
		rec, err := telemetry.NewFileRecorder(path)
		if err != nil {
			log.Fatalf("telemetry: %v", err)
		}
		defer rec.Close()
		server.AddReceivingMiddleware(telemetry.Middleware(rec))
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
