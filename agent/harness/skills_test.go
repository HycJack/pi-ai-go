package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSkillsFromDir(t *testing.T) {
	// Create temp dir with skill files
	dir := t.TempDir()

	skillContent := `---
name: test-skill
description: A test skill
---
This is the skill content.
Instructions for the model.`

	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillContent), 0644)

	skills, diags := LoadSkills(dir)
	if len(diags) > 0 {
		for _, d := range diags {
			t.Logf("diag: %s: %s", d.Code, d.Message)
		}
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Name != "test-skill" {
		t.Errorf("expected name test-skill, got %q", s.Name)
	}
	if s.Description != "A test skill" {
		t.Errorf("expected description, got %q", s.Description)
	}
	if s.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestLoadSkillsRecursive(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	os.MkdirAll(subdir, 0755)

	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: root-skill\n---\nroot"), 0644)
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("---\nname: sub-skill\n---\nsub"), 0644)

	skills, _ := LoadSkills(dir)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestLoadSkillsSkipsHidden(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".hidden")
	os.MkdirAll(hidden, 0755)
	os.WriteFile(filepath.Join(hidden, "SKILL.md"), []byte("hidden"), 0644)

	skills, _ := LoadSkills(dir)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (hidden dir), got %d", len(skills))
	}
}

func TestFormatSkillsForSystemPrompt(t *testing.T) {
	skills := []Skill{
		{Name: "coding", Description: "Help with code", Content: "...", FilePath: "/tmp/coding/SKILL.md"},
		{Name: "hidden", Description: "Hidden skill", Content: "...", FilePath: "/tmp/hidden/SKILL.md", DisableModelInvocation: true},
	}

	output := FormatSkillsForSystemPrompt(skills)
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	if !contains(output, "coding") {
		t.Error("expected 'coding' in output")
	}
	if contains(output, "hidden") {
		t.Error("expected 'hidden' to be excluded")
	}
}

func TestFormatInvocation(t *testing.T) {
	skill := Skill{Name: "test", Content: "do something", FilePath: "/a/b/SKILL.md"}
	output := FormatInvocation(skill, "extra instructions")
	if !contains(output, "do something") {
		t.Error("expected skill content")
	}
	if !contains(output, "extra instructions") {
		t.Error("expected additional instructions")
	}
}

func TestFormatTemplateInvocation(t *testing.T) {
	tmpl := PromptTemplate{
		Name:    "greet",
		Content: "Hello {{name}}, welcome to {{place}}!",
	}
	result := FormatTemplateInvocation(tmpl, map[string]string{"name": "Alice", "place": "Wonderland"})
	if result != "Hello Alice, welcome to Wonderland!" {
		t.Errorf("unexpected result: %q", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
