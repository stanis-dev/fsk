// Command fiskaly-mcp serves the embedded fiskaly SIGN IT documentation corpus over MCP.
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
		defer func() {
			if err := rec.Close(); err != nil {
				log.Printf("telemetry: close failed: %v", err)
			}
		}()
		server.AddReceivingMiddleware(telemetry.Middleware(rec))
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
