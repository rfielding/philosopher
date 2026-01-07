package main

// MCP Tool Definitions
// These are exposed to the LLM as callable tools

var mcpTools = []map[string]interface{}{
	{
		"name": "state_diagram",
		"description": "Render an actor's state machine as a mermaid diagram. Returns mermaid code that will be displayed to the user.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"actor": map[string]interface{}{
					"type":        "string",
					"description": "Name of the actor to render",
				},
			},
			"required": []string{"actor"},
		},
	},
	{
		"name": "sequence_diagram",
		"description": "Render message flow between actors as a mermaid sequence diagram. Queries the message log for actual sent/received messages.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"actors": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated list of actor names to include",
				},
				"time_range": map[string]interface{}{
					"type":        "string",
					"description": "Optional: 'last-10' or 'all' or '5-15' for time range",
				},
			},
			"required": []string{"actors"},
		},
	},
	{
		"name": "property",
		"description": "Check a temporal property (CTL formula) against the current state. Returns whether the property holds.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Human-readable name for the property",
				},
				"formula": map[string]interface{}{
					"type":        "string",
					"description": "CTL formula like 'AG(inventory >= 0)' or 'EF(sale completed)'",
				},
			},
			"required": []string{"formula"},
		},
	},
	{
		"name": "facts_table",
		"description": "Query collected facts and display as a table. Use this to show the user what facts have been collected.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"predicate": map[string]interface{}{
					"type":        "string",
					"description": "Fact predicate to query, e.g. 'sale' or 'inventory'",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Max rows to show (default 10)",
				},
			},
			"required": []string{"predicate"},
		},
	},
	{
		"name": "metrics_chart",
		"description": "Render time-series metrics as an xychart. Queries the metrics registry.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Chart title",
				},
				"metrics": map[string]interface{}{
					"type":        "string",
					"description": "Comma-separated metric names to plot",
				},
			},
			"required": []string{"metrics"},
		},
	},
	{
		"name": "define_actor",
		"description": "Define an actor with states and transitions. This creates the actor in the system.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Actor name",
				},
				"initial_state": map[string]interface{}{
					"type":        "string",
					"description": "Starting state name",
				},
				"states": map[string]interface{}{
					"type":        "array",
					"description": "List of state definitions",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type": "string",
							},
							"on_enter": map[string]interface{}{
								"type":        "string",
								"description": "Action when entering state",
							},
						},
					},
				},
				"transitions": map[string]interface{}{
					"type":        "array",
					"description": "List of transitions",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"from": map[string]interface{}{
								"type": "string",
							},
							"to": map[string]interface{}{
								"type": "string",
							},
							"guard": map[string]interface{}{
								"type":        "string",
								"description": "Condition: 'recv msg-type' or 'timeout 100'",
							},
							"action": map[string]interface{}{
								"type":        "string",
								"description": "Effect: 'send-to actor msg' or 'set var value'",
							},
						},
					},
				},
			},
			"required": []string{"name", "initial_state", "transitions"},
		},
	},
	{
		"name": "define_rule",
		"description": "Define a Datalog rule for deriving facts. Use for detecting patterns like deadlock.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Rule name",
				},
				"head": map[string]interface{}{
					"type":        "string",
					"description": "Derived fact pattern, e.g. 'deadlock ?a ?b'",
				},
				"body": map[string]interface{}{
					"type":        "array",
					"description": "Conditions that must all match",
					"items": map[string]interface{}{
						"type":        "string",
						"description": "Fact pattern, e.g. 'waiting-for ?a ?b'",
					},
				},
			},
			"required": []string{"name", "head", "body"},
		},
	},
	{
		"name": "run_simulation",
		"description": "Run the actor system for N steps. Collects facts during execution.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"steps": map[string]interface{}{
					"type":        "integer",
					"description": "Number of simulation steps",
				},
				"seed": map[string]interface{}{
					"type":        "integer",
					"description": "Optional random seed for reproducibility",
				},
			},
			"required": []string{"steps"},
		},
	},
	{
		"name": "tla_spec",
		"description": "Generate TLA+ specification from an actor. For users who want formal TLA+ output.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"actor": map[string]interface{}{
					"type":        "string",
					"description": "Actor name to translate",
				},
			},
			"required": []string{"actor"},
		},
	},
	{
		"name": "alloy_spec",
		"description": "Generate Alloy specification from an actor. For users who want Alloy output.",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"actor": map[string]interface{}{
					"type":        "string",
					"description": "Actor name to translate",
				},
			},
			"required": []string{"actor"},
		},
	},
}

// Example of what LLM would generate (for documentation):
const exampleLLMOutput = `
## StoreFront Actor

This actor manages inventory and processes customer orders.

{{state_diagram actor="storefront"}}

## Message Protocol

Here's how messages flow between production and sales:

{{sequence_diagram actors="production,storefront,customer"}}

## Verified Properties

{{property name="Inventory Safety" formula="AG(inventory >= 0)"}}
{{property name="No Deadlock" formula="AG(not deadlock)"}}

## Collected Facts

{{facts_table predicate="sale" limit=5}}

## Performance Metrics

{{metrics_chart title="7-Day Sales" metrics="produced,sold,inventory"}}
`
