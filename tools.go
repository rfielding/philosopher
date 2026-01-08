package main

import (
	"fmt"
	"regexp"
	"strings"
)

// ToolRegistry holds all available template tools
type ToolRegistry struct {
	ev    *Evaluator
	tools map[string]ToolFunc
}

// ToolFunc processes a tool call and returns rendered output
type ToolFunc func(ev *Evaluator, args map[string]string) string

// NewToolRegistry creates registry with all built-in tools
func NewToolRegistry(ev *Evaluator) *ToolRegistry {
	tr := &ToolRegistry{
		ev:    ev,
		tools: make(map[string]ToolFunc),
	}
	
	// Register all tools
	tr.tools["state_diagram"] = toolStateDiagram
	tr.tools["sequence_diagram"] = toolSequenceDiagram
	tr.tools["property"] = toolProperty
	tr.tools["facts_table"] = toolFactsTable
	tr.tools["metrics_chart"] = toolMetricsChart
	tr.tools["tla_spec"] = toolTLASpec
	tr.tools["alloy_spec"] = toolAlloySpec
	
	return tr
}

// Process finds {{tool args}} and substitutes with rendered output
func (tr *ToolRegistry) Process(markdown string) string {
	// Pattern: {{tool_name key="value" key2="value2"}}
	re := regexp.MustCompile(`\{\{(\w+)([^}]*)\}\}`)
	
	return re.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match // Leave unchanged if can't parse
		}
		
		toolName := parts[1]
		argsStr := ""
		if len(parts) > 2 {
			argsStr = parts[2]
		}
		
		args := parseToolArgs(argsStr)
		
		if tool, ok := tr.tools[toolName]; ok {
			return tool(tr.ev, args)
		}
		
		return fmt.Sprintf("<!-- Unknown tool: %s -->", toolName)
	})
}

// parseToolArgs extracts key="value" pairs
func parseToolArgs(s string) map[string]string {
	args := make(map[string]string)
	re := regexp.MustCompile(`(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) == 3 {
			args[m[1]] = m[2]
		}
	}
	return args
}

// ============================================================
// TOOL IMPLEMENTATIONS
// ============================================================

// toolStateDiagram renders actor state machine as mermaid
func toolStateDiagram(ev *Evaluator, args map[string]string) string {
	actorName := args["actor"]
	if actorName == "" {
		return "<!-- state_diagram: missing actor arg -->"
	}
	
	// Look up actor in registry
	actorDef, exists := ev.Registry[actorName]
	if !exists {
		return fmt.Sprintf("<!-- state_diagram: actor '%s' not found -->", actorName)
	}
	_ = actorDef // will use later
	
	// TODO: Extract states and transitions from actor definition
	// For now, return placeholder
	
	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("stateDiagram-v2\n")
	sb.WriteString(fmt.Sprintf("    [*] --> %s_Initial\n", actorName))
	sb.WriteString(fmt.Sprintf("    note right of %s_Initial\n", actorName))
	sb.WriteString(fmt.Sprintf("        Actor: %s\n", actorName))
	sb.WriteString("        (states extracted from definition)\n")
	sb.WriteString("    end note\n")
	sb.WriteString("```\n")
	
	return sb.String()
}

// toolSequenceDiagram renders message flow as mermaid
func toolSequenceDiagram(ev *Evaluator, args map[string]string) string {
	actors := args["actors"] // comma-separated
	if actors == "" {
		return "<!-- sequence_diagram: missing actors arg -->"
	}
	
	actorList := strings.Split(actors, ",")
	
	// Query message facts from Datalog
	// (sent ?from ?to ?msg ?time)
	results := ev.DatalogDB.Query("sent", Var("from"), Var("to"), Var("msg"), Var("time"))
	
	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("sequenceDiagram\n")
	
	// Declare participants
	for _, a := range actorList {
		a = strings.TrimSpace(a)
		sb.WriteString(fmt.Sprintf("    participant %s\n", a))
	}
	
	// Add messages from facts
	for _, binding := range results {
		from := termToString(binding["from"])
		to := termToString(binding["to"])
		msg := termToString(binding["msg"])
		sb.WriteString(fmt.Sprintf("    %s->>%s: %s\n", from, to, msg))
	}
	
	if len(results) == 0 {
		sb.WriteString("    Note over " + actorList[0] + ": No messages recorded yet\n")
	}
	
	sb.WriteString("```\n")
	return sb.String()
}

// toolProperty renders CTL property check result
func toolProperty(ev *Evaluator, args map[string]string) string {
	formula := args["formula"]
	name := args["name"]
	if formula == "" {
		return "<!-- property: missing formula arg -->"
	}
	
	// Parse and evaluate the formula
	// For now, just format it nicely
	
	var result string
	var resultClass string
	
	// Check if it's a temporal operator we can evaluate
	if strings.HasPrefix(formula, "AG(") {
		// Extract inner pattern and use never? or always?
		// TODO: Actually evaluate
		result = "checking..."
		resultClass = "pending"
	} else {
		result = "?"
		resultClass = "unknown"
	}
	
	// Return formatted property box (HTML)
	if name == "" {
		name = formula
	}
	
	return fmt.Sprintf(`<div class="property-box %s">
  <span class="property-name">%s</span>
  <code class="property-formula">%s</code>
  <span class="property-result">%s</span>
</div>`, resultClass, name, formula, result)
}

// toolFactsTable renders Datalog query results as table
func toolFactsTable(ev *Evaluator, args map[string]string) string {
	predicate := args["predicate"]
	if predicate == "" {
		// No predicate = show summary
		return toolFactsSummary(ev, args)
	}
	
	limit := 10 // default
	if l, ok := args["limit"]; ok {
		fmt.Sscanf(l, "%d", &limit)
	}
	
	// Filter facts by predicate
	var matchingFacts []Fact
	for _, fact := range ev.DatalogDB.Facts {
		if fact.Predicate == predicate {
			matchingFacts = append(matchingFacts, fact)
		}
	}
	
	if len(matchingFacts) == 0 {
		return fmt.Sprintf("*No facts for predicate `%s`*", predicate)
	}
	
	var sb strings.Builder
	sb.WriteString("| # | Fact |\n")
	sb.WriteString("|---|------|\n")
	
	count := 0
	for _, fact := range matchingFacts {
		if count >= limit {
			break
		}
		sb.WriteString(fmt.Sprintf("| %d | %s |\n", count+1, formatFact(fact)))
		count++
	}
	
	if len(matchingFacts) > limit {
		sb.WriteString(fmt.Sprintf("\n*...and %d more*\n", len(matchingFacts)-limit))
	}
	
	return sb.String()
}

// toolFactsSummary shows counts by predicate
func toolFactsSummary(ev *Evaluator, args map[string]string) string {
	if len(ev.DatalogDB.Facts) == 0 {
		return "*No facts collected yet*"
	}
	
	// Count by predicate
	counts := make(map[string]int)
	for _, fact := range ev.DatalogDB.Facts {
		counts[fact.Predicate]++
	}
	
	var sb strings.Builder
	sb.WriteString("| Predicate | Count |\n")
	sb.WriteString("|-----------|-------|\n")
	
	total := 0
	for pred, count := range counts {
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", pred, count))
		total += count
	}
	
	sb.WriteString(fmt.Sprintf("| **Total** | **%d** |\n", total))
	
	return sb.String()
}

// toolMetricsChart renders xychart from fact data
// Usage: {{metrics_chart title="X" predicates="sent,received"}}
// Or: {{metrics_chart title="X" predicate="sale" field="2"}}
func toolMetricsChart(ev *Evaluator, args map[string]string) string {
	title := args["title"]
	
	var sb strings.Builder
	sb.WriteString("```mermaid\n")
	sb.WriteString("xychart-beta\n")
	if title != "" {
		sb.WriteString(fmt.Sprintf("    title \"%s\"\n", title))
	}
	
	// Get max time for x-axis
	maxTime := int64(0)
	for _, fact := range ev.DatalogDB.Facts {
		if fact.Time > maxTime {
			maxTime = fact.Time
		}
	}
	
	if maxTime == 0 {
		sb.WriteString("    x-axis [0]\n")
		sb.WriteString("    y-axis \"Count\" 0 --> 10\n")
		sb.WriteString("    line \"no data\" [0]\n")
		sb.WriteString("```\n")
		return sb.String()
	}
	
	// Build x-axis labels
	xLabels := make([]string, 0)
	step := maxTime / 10
	if step < 1 {
		step = 1
	}
	for t := int64(0); t <= maxTime; t += step {
		xLabels = append(xLabels, fmt.Sprintf("%d", t))
	}
	sb.WriteString(fmt.Sprintf("    x-axis [%s]\n", strings.Join(xLabels, ", ")))
	
	// Check for predicates to count over time
	if predicates := args["predicates"]; predicates != "" {
		predList := strings.Split(predicates, ",")
		
		maxY := 0
		seriesData := make(map[string][]int)
		
		for _, pred := range predList {
			pred = strings.TrimSpace(pred)
			// Count facts per time bucket
			counts := make([]int, len(xLabels))
			cumulative := 0
			
			bucketIdx := 0
			for t := int64(0); t <= maxTime && bucketIdx < len(counts); t += step {
				// Count facts in this time range
				for _, fact := range ev.DatalogDB.Facts {
					if fact.Predicate == pred && fact.Time >= t && fact.Time < t+step {
						cumulative++
					}
				}
				counts[bucketIdx] = cumulative
				if cumulative > maxY {
					maxY = cumulative
				}
				bucketIdx++
			}
			seriesData[pred] = counts
		}
		
		sb.WriteString(fmt.Sprintf("    y-axis \"Count\" 0 --> %d\n", maxY+10))
		
		for pred, counts := range seriesData {
			countStrs := make([]string, len(counts))
			for i, c := range counts {
				countStrs[i] = fmt.Sprintf("%d", c)
			}
			sb.WriteString(fmt.Sprintf("    line \"%s\" [%s]\n", pred, strings.Join(countStrs, ", ")))
		}
	} else {
		// Fallback: show counts of all predicates
		sb.WriteString("    y-axis \"Count\" 0 --> 100\n")
		sb.WriteString("    line \"facts\" [0]\n")
	}
	
	sb.WriteString("```\n")
	return sb.String()
}

// toolTLASpec renders actor as TLA+ specification
func toolTLASpec(ev *Evaluator, args map[string]string) string {
	actorName := args["actor"]
	if actorName == "" {
		return "<!-- tla_spec: missing actor arg -->"
	}
	
	// TODO: Actually translate actor definition to TLA+
	
	var sb strings.Builder
	sb.WriteString("```tla\n")
	sb.WriteString(fmt.Sprintf("---- MODULE %s ----\n", actorName))
	sb.WriteString("EXTENDS Naturals, Sequences\n\n")
	sb.WriteString("VARIABLES state, inbox, outbox\n\n")
	sb.WriteString("Init ==\n")
	sb.WriteString("    /\\ state = \"Idle\"\n")
	sb.WriteString("    /\\ inbox = <<>>\n")
	sb.WriteString("    /\\ outbox = <<>>\n\n")
	sb.WriteString("Next ==\n")
	sb.WriteString("    \\/ ReceiveMessage\n")
	sb.WriteString("    \\/ ProcessMessage\n\n")
	sb.WriteString("(* Generated from BoundedLISP actor definition *)\n")
	sb.WriteString("====\n")
	sb.WriteString("```\n")
	
	return sb.String()
}

// toolAlloySpec renders actor as Alloy specification
func toolAlloySpec(ev *Evaluator, args map[string]string) string {
	actorName := args["actor"]
	if actorName == "" {
		return "<!-- alloy_spec: missing actor arg -->"
	}
	
	var sb strings.Builder
	sb.WriteString("```alloy\n")
	sb.WriteString(fmt.Sprintf("module %s\n\n", actorName))
	sb.WriteString("sig State {}\n")
	sb.WriteString("one sig Idle, Processing extends State {}\n\n")
	sb.WriteString("sig Actor {\n")
	sb.WriteString("    state: one State,\n")
	sb.WriteString("    inbox: seq Message\n")
	sb.WriteString("}\n\n")
	sb.WriteString("sig Message {}\n\n")
	sb.WriteString("// Generated from BoundedLISP actor definition\n")
	sb.WriteString("```\n")
	
	return sb.String()
}

// ============================================================
// HELPERS
// ============================================================

func termToString(t Term) string {
	if t.IsVar {
		return "?" + t.Name
	}
	if t.IsNum {
		return fmt.Sprintf("%g", t.Num)
	}
	if t.IsStr {
		return t.Str
	}
	if t.IsList {
		parts := make([]string, len(t.List))
		for i, item := range t.List {
			parts[i] = termToString(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return t.Name // atom
}

func formatFact(f Fact) string {
	parts := make([]string, len(f.Args)+1)
	parts[0] = f.Predicate
	for i, a := range f.Args {
		parts[i+1] = termToString(a)
	}
	return strings.Join(parts, " ")
}
