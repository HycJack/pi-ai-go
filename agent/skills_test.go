package agent

import (
	"testing"

	session "pi-ai-go/agent/session"
)

func TestBuildSystemPromptWithSkills(t *testing.T) {
	tests := []struct {
		name           string
		config         AgentLoopConfig
		expectedHas    []string
		expectedNotHas []string
	}{
		{
			name: "no skills or templates",
			config: AgentLoopConfig{
				SystemPrompt: "Base prompt",
			},
			expectedHas:    []string{"Base prompt"},
			expectedNotHas: []string{"available_skills", "available_templates"},
		},
		{
			name: "with skills",
			config: AgentLoopConfig{
				SystemPrompt: "Base prompt",
				Skills: []session.Skill{
					{
						Name:        "TestSkill",
						Description: "A test skill",
						Content:     "Skill content",
						FilePath:    "/path/to/skill.md",
					},
				},
			},
			expectedHas:    []string{"Base prompt", "available_skills", "TestSkill", "A test skill"},
			expectedNotHas: []string{"available_templates"},
		},
		{
			name: "with templates",
			config: AgentLoopConfig{
				SystemPrompt: "Base prompt",
				PromptTemplates: []session.PromptTemplate{
					{
						Name:        "TestTemplate",
						Description: "A test template",
						Content:     "Template with {{name}}",
					},
				},
			},
			expectedHas:    []string{"Base prompt", "available_templates", "TestTemplate"},
			expectedNotHas: []string{"available_skills"},
		},
		{
			name: "with skills and templates",
			config: AgentLoopConfig{
				SystemPrompt: "Base prompt",
				Skills: []session.Skill{
					{
						Name:        "TestSkill",
						Description: "A test skill",
						Content:     "Skill content",
						FilePath:    "/path/to/skill.md",
					},
				},
				PromptTemplates: []session.PromptTemplate{
					{
						Name:        "TestTemplate",
						Description: "A test template",
						Content:     "Template with {{name}}",
					},
				},
			},
			expectedHas:    []string{"Base prompt", "available_skills", "available_templates", "TestSkill", "TestTemplate"},
			expectedNotHas: []string{},
		},
		{
			name: "skill with disable model invocation",
			config: AgentLoopConfig{
				SystemPrompt: "Base prompt",
				Skills: []session.Skill{
					{
						Name:                   "HiddenSkill",
						Description:            "A hidden skill",
						Content:                "Hidden content",
						FilePath:               "/path/to/hidden.md",
						DisableModelInvocation: true,
					},
				},
			},
			expectedHas:    []string{"Base prompt"},
			expectedNotHas: []string{"available_skills", "HiddenSkill"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSystemPromptWithSkills(tt.config)

			for _, has := range tt.expectedHas {
				if !contains(result, has) {
					t.Errorf("Expected prompt to contain %q, got %q", has, result)
				}
			}

			for _, notHas := range tt.expectedNotHas {
				if contains(result, notHas) {
					t.Errorf("Expected prompt NOT to contain %q, got %q", notHas, result)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
