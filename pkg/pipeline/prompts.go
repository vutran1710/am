package pipeline

import (
	"fmt"

	"github.com/vutran/agent-mesh/pkg/silo"
)

func standardizeSystemPrompt(profile, instructions string) string {
	base := `You are a message standardizer. Your job is to take raw messages from various sources (email, Slack, Discord, calendar) and extract structured information.

Always respond with valid JSON only, no markdown or explanation.`

	if profile != "" {
		base += "\n\n## User Profile\n" + profile
	}
	if instructions != "" {
		base += "\n\n## Instructions\n" + instructions
	}
	return base
}

func standardizeUserPrompt(source, sender, subject, preview, raw string) string {
	return fmt.Sprintf(`Standardize this message. Return JSON with these fields:
- clean_text: the message content stripped of HTML, signatures, boilerplate
- from_name: sender's human name
- from_email: sender's email if available
- action: one of "request", "info", "reminder", "question", "invite", "update"
- deadline: extracted deadline if mentioned (ISO 8601 or natural language), empty string if none
- summary: 1-2 sentence summary of what this message is about

Source: %s
Sender: %s
Subject: %s
Preview: %s

Raw:
%s`, source, sender, subject, preview, truncate(raw, 2000))
}

func evaluateSystemPrompt(profile, instructions string) string {
	base := `You are a message evaluator. Given a standardized message, assess its importance and relevance to the user.

Always respond with valid JSON only, no markdown or explanation.`

	if profile != "" {
		base += "\n\n## User Profile\n" + profile
	}
	if instructions != "" {
		base += "\n\n## Instructions\n" + instructions
	}
	return base
}

func evaluateUserPrompt(std *silo.Standardized, source string) string {
	return fmt.Sprintf(`Evaluate this message. Return JSON with these fields:
- importance: integer 1-10 (10 = critical, 1 = noise)
- category: one of "work", "personal", "spam", "notification", "social"
- action_needed: boolean, does the user need to do something?
- urgency: one of "now", "today", "this_week", "none"
- reason: 1 sentence explaining the score

Source: %s
From: %s <%s>
Action type: %s
Deadline: %s
Summary: %s

Content:
%s`, source, std.FromName, std.FromEmail, std.Action, std.Deadline, std.Summary, truncate(std.CleanText, 1000))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
