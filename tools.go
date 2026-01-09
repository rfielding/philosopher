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
	tr.tools["properties"] = toolPropertiesTable
	tr.tools["facts_table"] = toolFactsTable
	tr.tools["facts_list"] = toolFactsList
	tr.tools["metrics_chart"] = toolMetricsChart
	tr.tools["tla_spec"] = toolTLASpec
	tr.tools["alloy_spec"] = toolAlloySpec
	
	return tr
}

// Process finds {{tool args}} and substitutes with rendered output
func (tr *ToolRegistry) Process(markdown string) string {
	// Pattern: {{tool_name key="value" key2="value2"}}
	re := regexp.MustCompile(`\{\{(\w+)([^}]*)\}\}`)
	
	result := re.ReplaceAllStringFunc(markdown, func(match string) string {
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
	
	// Clean up mermaid blocks
	return cleanMermaidBlocks(result)
}

// cleanMermaidBlocks fixes common mermaid syntax issues
func cleanMermaidBlocks(markdown string) string {
	// Find mermaid code blocks and clean each one
	mermaidRe := regexp.MustCompile("(?s)```mermaid\n(.*?)```")
	
	return mermaidRe.ReplaceAllStringFunc(markdown, func(block string) string {
		// Extract content
		content := mermaidRe.FindStringSubmatch(block)
		if len(content) < 2 {
			return block
		}
		
		cleaned := cleanMermaidSyntax(content[1])
		return "```mermaid\n" + cleaned + "```"
	})
}

// cleanMermaidSyntax fixes specific syntax issues
func cleanMermaidSyntax(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	
	for _, line := range lines {
		cleaned := line
		
		// Fix common label issues in stateDiagram
		// Replace := with = (mermaid doesn't like :=)
		cleaned = strings.ReplaceAll(cleaned, ":=", "=")
		
		// Fix labels with problematic characters
		// Pattern: State --> State: label with bad chars
		if strings.Contains(cleaned, "-->") || strings.Contains(cleaned, "->") {
			// Remove or escape characters that break mermaid
			// Colons in labels (except the first one after state name)
			parts := strings.SplitN(cleaned, ": ", 2)
			if len(parts) == 2 {
				// Clean the label part
				label := parts[1]
				label = strings.ReplaceAll(label, ":", "-")
				label = strings.ReplaceAll(label, ";", ",")
				label = strings.ReplaceAll(label, "\"", "'")
				// Remove angle brackets and their content markers
				label = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(label, "")
				label = strings.ReplaceAll(label, "<", "")
				label = strings.ReplaceAll(label, ">", "")
				cleaned = parts[0] + ": " + strings.TrimSpace(label)
			}
		}
		
		// Fix sequence diagram issues
		// ->> should not have special chars in message
		if strings.Contains(cleaned, "->>") || strings.Contains(cleaned, "-->>") {
			parts := strings.SplitN(cleaned, ": ", 2)
			if len(parts) == 2 {
				label := parts[1]
				label = strings.ReplaceAll(label, "\"", "'")
				// Remove angle brackets and content
				label = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(label, "")
				label = strings.ReplaceAll(label, "<", "")
				label = strings.ReplaceAll(label, ">", "")
				cleaned = parts[0] + ": " + strings.TrimSpace(label)
			}
		}
		
		result = append(result, cleaned)
	}
	
	return strings.Join(result, "\n")
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
	
	if name == "" {
		name = formula
	}
	
	// Parse the formula and evaluate it
	// Format: "always? '(pred args)" or "eventually? '(pred args)" etc.
	var result bool
	var evaluated bool
	
	formula = strings.TrimSpace(formula)
	
	// Try to parse and evaluate
	if strings.HasPrefix(formula, "always?") || strings.HasPrefix(formula, "AG") {
		inner := extractInner(formula)
		if inner != "" {
			goal := parseGoalFromString(inner)
			if goal.Predicate != "" {
				result = ev.DatalogDB.Always(goal)
				evaluated = true
			}
		}
	} else if strings.HasPrefix(formula, "eventually?") || strings.HasPrefix(formula, "AF") {
		inner := extractInner(formula)
		if inner != "" {
			goal := parseGoalFromString(inner)
			if goal.Predicate != "" {
				result = ev.DatalogDB.Eventually(goal)
				evaluated = true
			}
		}
	} else if strings.HasPrefix(formula, "never?") || strings.HasPrefix(formula, "AG(not") || strings.HasPrefix(formula, "AG(¬") {
		inner := extractInner(formula)
		if inner != "" {
			goal := parseGoalFromString(inner)
			if goal.Predicate != "" {
				result = ev.DatalogDB.Never(goal)
				evaluated = true
			}
		}
	} else if strings.HasPrefix(formula, "possibly?") || strings.HasPrefix(formula, "EF") {
		inner := extractInner(formula)
		if inner != "" {
			goal := parseGoalFromString(inner)
			if goal.Predicate != "" {
				result = ev.DatalogDB.Possibly(goal)
				evaluated = true
			}
		}
	}
	
	// Format result
	var resultStr, icon string
	if !evaluated {
		resultStr = "?"
		icon = "❓"
	} else if result {
		resultStr = "✓ true"
		icon = "✅"
	} else {
		resultStr = "✗ false"
		icon = "❌"
	}
	
	return fmt.Sprintf("| %s | `%s` | %s %s |", name, formula, icon, resultStr)
}

// extractInner pulls the predicate pattern from formulas like "always? '(pred args)"
func extractInner(formula string) string {
	// Find the quoted list
	start := strings.Index(formula, "'(")
	if start == -1 {
		start = strings.Index(formula, "(")
		if start == -1 {
			return ""
		}
	} else {
		start++ // skip the quote
	}
	
	// Find matching close paren
	depth := 0
	for i := start; i < len(formula); i++ {
		if formula[i] == '(' {
			depth++
		} else if formula[i] == ')' {
			depth--
			if depth == 0 {
				return formula[start : i+1]
			}
		}
	}
	return ""
}

// parseGoalFromString parses "(pred arg1 arg2)" into a Goal
func parseGoalFromString(s string) Goal {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")
	
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return Goal{}
	}
	
	goal := Goal{Predicate: parts[0]}
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "?") {
			goal.Args = append(goal.Args, Var(p))
		} else {
			goal.Args = append(goal.Args, Atom(p))
		}
	}
	return goal
}

// toolPropertiesTable renders multiple property checks as a table
func toolPropertiesTable(ev *Evaluator, args map[string]string) string {
	var sb strings.Builder
	sb.WriteString("| Property | Formula | Result |\n")
	sb.WriteString("|----------|---------|--------|\n")
	
	// If specific properties provided, check those
	if props := args["checks"]; props != "" {
		for _, p := range strings.Split(props, ";") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			// Parse "name: formula" or just "formula"
			var name, formula string
			if idx := strings.Index(p, ":"); idx > 0 {
				name = strings.TrimSpace(p[:idx])
				formula = strings.TrimSpace(p[idx+1:])
			} else {
				formula = p
				name = p
			}
			row := toolProperty(ev, map[string]string{"name": name, "formula": formula})
			sb.WriteString(row)
			sb.WriteString("\n")
		}
	} else {
		// Default: show common checks
		checks := []struct{ name, formula string }{
			{"Messages sent", "eventually? '(sent ?from ?to ?msg)"},
			{"All actors spawned", "eventually? '(spawned ?name)"},
		}
		for _, c := range checks {
			row := toolProperty(ev, map[string]string{"name": c.name, "formula": c.formula})
			sb.WriteString(row)
			sb.WriteString("\n")
		}
	}
	
	return sb.String()
}

// toolFactsList shows actual facts (not just counts)
func toolFactsList(ev *Evaluator, args map[string]string) string {
	predicate := args["predicate"]
	limit := 20
	if l, ok := args["limit"]; ok {
		fmt.Sscanf(l, "%d", &limit)
	}
	
	var sb strings.Builder
	
	// Collect facts to show
	var facts []Fact
	for _, f := range ev.DatalogDB.Facts {
		if predicate == "" || f.Predicate == predicate {
			facts = append(facts, f)
			if len(facts) >= limit {
				break
			}
		}
	}
	
	if len(facts) == 0 {
		sb.WriteString("*No facts found.*\n")
		return sb.String()
	}
	
	// Group by predicate for cleaner output
	byPred := make(map[string][]Fact)
	for _, f := range facts {
		byPred[f.Predicate] = append(byPred[f.Predicate], f)
	}
	
	for pred, pFacts := range byPred {
		sb.WriteString(fmt.Sprintf("**%s** (%d):\n", pred, len(pFacts)))
		for _, f := range pFacts {
			args := make([]string, len(f.Args))
			for i, a := range f.Args {
				args[i] = a.String()
			}
			if f.Time > 0 {
				sb.WriteString(fmt.Sprintf("- `(%s %s)` @ t=%d\n", f.Predicate, strings.Join(args, " "), f.Time))
			} else {
				sb.WriteString(fmt.Sprintf("- `(%s %s)`\n", f.Predicate, strings.Join(args, " ")))
			}
		}
		sb.WriteString("\n")
	}
	
	if len(ev.DatalogDB.Facts) > limit {
		sb.WriteString(fmt.Sprintf("*... and %d more facts*\n", len(ev.DatalogDB.Facts)-limit))
	}
	
	return sb.String()
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
		sb.WriteString("\n⚠️ **No simulation data.** Did you forget `(run-scheduler N)`?\n")
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
	
	// Collect available predicates with counts
	availablePreds := make(map[string]int)
	for _, fact := range ev.DatalogDB.Facts {
		availablePreds[fact.Predicate]++
	}
	
	// Determine which predicates to chart
	var predList []string
	if predicates := args["predicates"]; predicates != "" {
		// Check if specified predicates exist
		specified := strings.Split(predicates, ",")
		for _, p := range specified {
			p = strings.TrimSpace(p)
			if _, exists := availablePreds[p]; exists {
				predList = append(predList, p)
			}
		}
		
		// If none matched, use all available (excluding spawned which is usually just initial)
		if len(predList) == 0 {
			for p := range availablePreds {
				if p != "spawned" {
					predList = append(predList, p)
				}
			}
			// Add note about auto-detection
			if len(predList) > 0 {
				sb.WriteString(fmt.Sprintf("    %%Note: auto-detected predicates (requested %s not found)\n", predicates))
			}
		}
	} else {
		// No predicates specified - use all except spawned
		for p := range availablePreds {
			if p != "spawned" {
				predList = append(predList, p)
			}
		}
	}
	
	if len(predList) == 0 {
		sb.WriteString("    y-axis \"Count\" 0 --> 10\n")
		sb.WriteString("    line \"no data\" [0]\n")
		sb.WriteString("```\n")
		sb.WriteString("\n⚠️ **No chartable predicates found.**\n")
		return sb.String()
	}
	
	// Count facts per time bucket for each predicate
	maxY := 0
	seriesData := make(map[string][]int)
	
	for _, pred := range predList {
		counts := make([]int, len(xLabels))
		cumulative := 0
		
		bucketIdx := 0
		for t := int64(0); t <= maxTime && bucketIdx < len(counts); t += step {
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
	
	if maxY == 0 {
		maxY = 10
	}
	sb.WriteString(fmt.Sprintf("    y-axis \"Count\" 0 --> %d\n", maxY+10))
	
	for pred, counts := range seriesData {
		countStrs := make([]string, len(counts))
		for i, c := range counts {
			countStrs[i] = fmt.Sprintf("%d", c)
		}
		sb.WriteString(fmt.Sprintf("    line \"%s\" [%s]\n", pred, strings.Join(countStrs, ", ")))
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
