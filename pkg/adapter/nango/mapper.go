package nango

import (
	"encoding/json"

	"github.com/vutran/agent-mesh/pkg/silo"
)

// FieldMapper extracts silo.Message fields from a Nango record's data.
// Each function receives the raw JSON data and returns the extracted value.
type FieldMapper struct {
	Sender  func(data json.RawMessage) string
	Subject func(data json.RawMessage) string
	Preview func(data json.RawMessage) string
}

// jsonString extracts a top-level string field from JSON.
func jsonString(data json.RawMessage, key string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

// StringField returns a mapper function that extracts a string field by key.
func StringField(key string) func(json.RawMessage) string {
	return func(data json.RawMessage) string {
		return jsonString(data, key)
	}
}

// ServiceConfig defines how to poll a specific Nango integration.
type ServiceConfig struct {
	Source            silo.Source
	ProviderConfigKey string // Nango integration ID (e.g. "google-mail")
	Model             string // Nango data model (e.g. "emails")
	Mapper            FieldMapper
}

// Pre-built service configs

var GmailConfig = ServiceConfig{
	Source:            silo.SourceGmail,
	ProviderConfigKey: "google-mail",
	Model:             "emails",
	Mapper: FieldMapper{
		Sender:  StringField("from"),
		Subject: StringField("subject"),
		Preview: StringField("snippet"),
	},
}

var SlackConfig = ServiceConfig{
	Source:            silo.SourceSlack,
	ProviderConfigKey: "slack",
	Model:             "messages",
	Mapper: FieldMapper{
		Sender:  StringField("user"),
		Subject: StringField("channel"),
		Preview: StringField("text"),
	},
}

var GCalConfig = ServiceConfig{
	Source:            silo.SourceGCal,
	ProviderConfigKey: "google-calendar",
	Model:             "events",
	Mapper: FieldMapper{
		Sender:  StringField("organizer"),
		Subject: StringField("summary"),
		Preview: StringField("description"),
	},
}
