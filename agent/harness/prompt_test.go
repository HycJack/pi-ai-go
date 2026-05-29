package harness

import "testing"

func TestBuildSystemPrompt(t *testing.T) {
	config := SystemPromptConfig{
		BasePrompt: "You are a helpful assistant.",
		Skills: []Skill{
			{Name: "coding", Description: "Help with code", Content: "...", FilePath: "/tmp/SKILL.md"},
		},
		PromptTemplates: []PromptTemplate{
			{Name: "review", Description: "Code review", Content: "Review {{file}}"},
		},
		CustomSections: []string{"Custom section text."},
	}

	output := BuildSystemPrompt(config)
	if !containsSubstr(output, "helpful assistant") {
		t.Error("expected base prompt")
	}
	if !containsSubstr(output, "coding") {
		t.Error("expected skill name")
	}
	if !containsSubstr(output, "review") {
		t.Error("expected template name")
	}
	if !containsSubstr(output, "Custom section") {
		t.Error("expected custom section")
	}
}

func TestBuildSystemPromptEmpty(t *testing.T) {
	output := BuildSystemPrompt(SystemPromptConfig{})
	if output != "" {
		t.Errorf("expected empty, got %q", output)
	}
}
