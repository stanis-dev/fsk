# Eval sandbox image: runs the coder hermetically.
#
# Multi-stage so only the MCP *binary* (not the MCP or judge source, not research/)
# reaches the coder. The fixture is bind-mounted at runtime; nothing else from the
# host is visible. Auth is via CLAUDE_CODE_OAUTH_TOKEN passed at `docker run`.
#
# Builds on Docker Desktop (linux/arm64): node 22 + claude-code 2.1.177 install clean,
# the MCP binary lands at /usr/local/bin/fiskaly-mcp, and bind mounts propagate both
# ways from ~/.cache/fiskaly-eval. A full token-backed eval has not been run here.

# 1) Build the fiskaly MCP binary in isolation.
FROM golang:1.25-bookworm AS mcpbuild
WORKDIR /src
COPY mcp/ ./
RUN go build -o /fiskaly-mcp .

# 2) Coder runtime: Go (for the coder's own go build/test) + the claude CLI.
FROM golang:1.25-bookworm
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates curl gnupg \
 && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
 && apt-get install -y --no-install-recommends nodejs \
 && npm install -g @anthropic-ai/claude-code@2.1.177 \
 && apt-get clean && rm -rf /var/lib/apt/lists/*

# Only the binary crosses the boundary; no MCP source.
COPY --from=mcpbuild /fiskaly-mcp /usr/local/bin/fiskaly-mcp
RUN printf '{ "mcpServers": { "fiskaly": { "command": "/usr/local/bin/fiskaly-mcp" } } }\n' >/etc/mcp.json

COPY sims/evals/docker-entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

WORKDIR /work
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
