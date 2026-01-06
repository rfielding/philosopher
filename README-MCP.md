# MCP Support for Philosopher

## Usage

```bash
# Build
go build -o philosopher main.go

# Run modes
./philosopher              # Web UI (default)
./philosopher -repl        # Interactive REPL
./philosopher file.lisp    # Run file
./philosopher -mcp         # MCP server (stdio)
./philosopher -mcp-sse 3000  # MCP server (HTTP/SSE)
```

## Claude Desktop Config

Add to `~/.config/claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "philosopher": {
      "command": "/path/to/philosopher",
      "args": ["-mcp"]
    }
  }
}
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `eval_lisp` | Evaluate BoundedLISP code |
| `run_simulation` | Run scheduler for N steps |
| `spawn_actor` | Create actor with name, mailbox, initial state |
| `send_message` | Send message to actor |
| `get_metrics` | Get registry metrics |
| `get_actors` | Get actor state(s) |
| `reset` | Clear all actors |
| `csp_status` | Get CSP violations |
| `csp_enforce` | Enable/disable CSP enforcement |

## Example Session

Claude can now directly control simulations:

```
You: "Run BreadCo for 7 days and show revenue"

Claude uses:
  1. eval_lisp → load breadco.lisp definitions
  2. spawn_actor × 7 → create actors
  3. run_simulation 700 → run 7 days
  4. get_metrics → read revenue, inventory
  
Claude: "After 7 days: $189 revenue, 35 loaves inventory"
```

## CSP Enforcement

```
You: "Enable strict CSP and check for violations"

Claude uses:
  1. csp_enforce enabled=true strict=true
  2. run_simulation 100
  3. csp_status
  
Claude: "No violations - all actors follow guard-first pattern"
```

## Files

- `main.go` - Full implementation with MCP
- `breadco.lisp` - Multi-actor bakery simulation
- `README-MCP.md` - This file
