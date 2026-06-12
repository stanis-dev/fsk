// z2r-mcp is an action-taking MCP server for fiskaly SIGN IT: AI agents
// provision Italian merchant sandboxes, issue fiscal receipts against the
// real TEST API, and get judged on the compliance of everything they did.
//
//	claude mcp add zero-to-receipt -- go run ./cmd/z2r-mcp
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"z2r/internal/envfile"
	"z2r/internal/fiskaly"
	"z2r/internal/mcpserver"
)

func main() {
	log.SetOutput(os.Stderr) // stdout belongs to the MCP stdio transport

	baseURL := flag.String("base-url", fiskaly.TestBaseURL, "fiskaly API base URL (TEST or a local simulator; LIVE is rejected)")
	envPath := flag.String("env", ".env", "path to .env with FISKALY_API_KEY/FISKALY_API_SECRET")
	flag.Parse()

	envfile.Load(*envPath)
	key, secret := os.Getenv("FISKALY_API_KEY"), os.Getenv("FISKALY_API_SECRET")
	if key == "" || secret == "" {
		log.Fatal("FISKALY_API_KEY and FISKALY_API_SECRET must be set")
	}

	server, err := mcpserver.New(mcpserver.Config{BaseURL: *baseURL, Key: key, Secret: secret})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("zero-to-receipt MCP server starting (stdio) against %s", *baseURL)
	if err := server.MCP().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
