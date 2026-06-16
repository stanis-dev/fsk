// Command fiskaly-mcp is the fiskaly MCP server. It serves the curated SIGN IT
// documentation corpus through two read-only tools, search_fiskaly_docs and
// fetch_fiskaly_doc, so a consumer agent can ground its integration in the docs.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"fiskaly-mcp/corpus"
)

func main() {
	c, err := corpus.Load()
	if err != nil {
		log.Fatal(err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fiskaly", Version: "v0.1.0"}, nil)
	registerTools(server, c)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
