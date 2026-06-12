package session

import (
	"strings"
)

// SystemPromptConfig configures the system prompt builder.
type SystemPromptConfig struct {
	// BasePrompt is the core system prompt.
	BasePrompt string
	// Skills to include in the system prompt.
	Skills []Skill
	// PromptTemplates available for invocation.
	PromptTemplates []PromptTemplate
	// CustomSections are additional sections appended to the prompt.
	CustomSections []string
}

// BuildSystemPrompt constructs the full system prompt from configuration.
func BuildSystemPrompt(config SystemPromptConfig) string {
	var parts []string

	// Base prompt
	if config.BasePrompt != "" {
		parts = append(parts, config.BasePrompt)
	}

	// Skills section
	if skillsSection := FormatSkillsForSystemPrompt(config.Skills); skillsSection != "" {
		parts = append(parts, skillsSection)
	}

	// Prompt templates section
	if templatesSection := formatTemplatesForSystemPrompt(config.PromptTemplates); templatesSection != "" {
		parts = append(parts, templatesSection)
	}

	// Custom sections
	parts = append(parts, config.CustomSections...)

	return strings.Join(parts, "\n\n")
}

func formatTemplatesForSystemPrompt(templates []PromptTemplate) string {
	if len(templates) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "The following prompt templates are available:")
	lines = append(lines, "")
	lines = append(lines, "<available_templates>")

	for _, t := range templates {
		lines = append(lines, "  <template>")
		lines = append(lines, "    <name>"+escapeXML(t.Name)+"</name>")
		if t.Description != "" {
			lines = append(lines, "    <description>"+escapeXML(t.Description)+"</description>")
		}
		lines = append(lines, "  </template>")
	}

	lines = append(lines, "</available_templates>")
	return strings.Join(lines, "\n")
}
