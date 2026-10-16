package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPrompts(server *mcp.Server, session *Session) {
	server.AddPrompt(&mcp.Prompt{
		Name:        "integrate_app",
		Title:       "Add MotifEngine to this application",
		Description: "Find the business events in the codebase, configure the workspace, prove it with test events, and write the backend integration.",
		Arguments: []*mcp.PromptArgument{
			{Name: "goal", Description: "What the product should reward or measure, for example 'points for paid orders and a first-purchase badge'"},
			{Name: "environment", Description: "Environment to configure; defaults to the only one available"},
		},
	}, func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		goal := strings.TrimSpace(request.Params.Arguments["goal"])
		if goal == "" {
			goal = "decide with me, from what the code does, which actions are worth rewarding"
		}
		return userPrompt("Add MotifEngine to this application", fmt.Sprintf(`Integrate MotifEngine into this application. Goal: %s.%s

Work in this order and keep me informed:
1. Call get_workspace and read_docs topic quickstart. %s
2. Read the codebase to find where each relevant business fact becomes true on the server (payment captured, lesson graded). List them with file paths and propose event types and payload fields before changing anything. Rules read only the current event, so include every field a rule needs.
3. Register the event contracts, then create only the catalog items the design needs.
4. Write each rule, run check_rule with a matching and a non-matching sample payload, and save drafts.
5. Show me the rules and ask before publish_rule. If the token cannot publish, give me the review link.
6. Prove it: send_test_event with a test user, read the trace, then send the same event_id again and confirm nothing was awarded twice.
7. Use get_integration_snippet and add the sending code to the backend where the fact becomes true, with the key in MOTIF_API_KEY, a stable event_id and retries.
8. Finish with the launch checklist from read_docs and tell me what is left for production.`, goal, environmentNote(request), sandboxHint(session))), nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "design_program",
		Title:       "Design an engagement or loyalty program",
		Description: "Propose events, catalog and rules for a product and goal, without changing anything.",
		Arguments: []*mcp.PromptArgument{
			{Name: "product", Description: "What the product is and who uses it", Required: true},
			{Name: "goal", Description: "The behaviour to encourage, for example repeat purchases or daily practice", Required: true},
		},
	}, func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return userPrompt("Design a MotifEngine program", fmt.Sprintf(`Design a MotifEngine program for this product: %s
Goal: %s

Read the recipes, rule-language and boundaries docs first, and list what already exists with list_event_types, list_catalog and list_rules. Then propose, without creating anything yet:
- the event types and payload fields the backend should send (rules read only the current event),
- currencies, levels, missions, achievements and rewards, each with the reason it exists,
- each rule as event, conditions and actions, checked with check_rule against realistic sample payloads,
- what to measure with goals or campaigns.
Point out anything in the goal that the engine's boundaries do not support and how the backend should handle it instead.`, request.Params.Arguments["product"], request.Params.Arguments["goal"])), nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name:        "debug_event",
		Title:       "Explain what happened to an event",
		Description: "Trace one event and explain what it awarded, what failed and why a rule did or did not match.",
		Arguments: []*mcp.PromptArgument{
			{Name: "event_id", Description: "The event_id the producer sent", Required: true},
			{Name: "environment", Description: "Environment the event was sent to"},
		},
	}, func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return userPrompt("Explain an event", fmt.Sprintf(`Explain what happened to MotifEngine event %q.%s

Call get_event_trace. Report the processing status, each rule that ran and what it awarded, any failed execution with its last error, and for each rule that did not match, the exact expression that decided it. If the event never processed, check get_diagnostics. If the user's balance looks wrong, compare with get_user_state. Suggest the smallest fix, either in the producer's payload or in a new rule draft, and do not publish anything without asking.`, request.Params.Arguments["event_id"], environmentNote(request))), nil
	})
}

func userPrompt(description, text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{Description: description, Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}}}
}

func environmentNote(request *mcp.GetPromptRequest) string {
	if environment := strings.TrimSpace(request.Params.Arguments["environment"]); environment != "" {
		return " Use the " + environment + " environment."
	}
	return ""
}

func sandboxHint(session *Session) string {
	for _, environment := range session.Environments {
		if environment.ID != "production" {
			return "Build and test in a non-production environment such as " + environment.ID + "; repeat the configuration in production only when I say so."
		}
	}
	return "This token can only reach production, so treat every write and test event as live and confirm with me first."
}
