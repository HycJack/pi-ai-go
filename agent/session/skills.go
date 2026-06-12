package session

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// SkillDiagnostic is a warning produced while loading skills.
type SkillDiagnostic struct {
	Type    string // "warning"
	Code    ErrorCode
	Message string
	Path    string
}

// LoadSkills loads skills from one or more directories.
// It traverses directories recursively, loads SKILL.md files,
// and returns diagnostics for invalid skill files.
func LoadSkills(dirs ...string) ([]Skill, []SkillDiagnostic) {
	var skills []Skill
	var diagnostics []SkillDiagnostic

	for _, dir := range dirs {
		loaded, diags := loadSkillsFromDir(dir, dir)
		skills = append(skills, loaded...)
		diagnostics = append(diagnostics, diags...)
	}

	return skills, diagnostics
}

func loadSkillsFromDir(dir, rootDir string) ([]Skill, []SkillDiagnostic) {
	var skills []Skill
	var diagnostics []SkillDiagnostic

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		diagnostics = append(diagnostics, SkillDiagnostic{
			Type:    "warning",
			Code:    ErrNotFound,
			Message: err.Error(),
			Path:    dir,
		})
		return nil, diagnostics
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			sub, subDiags := loadSkillsFromDir(path, rootDir)
			skills = append(skills, sub...)
			diagnostics = append(diagnostics, subDiags...)
			continue
		}

		// Load SKILL.md files
		if strings.EqualFold(entry.Name(), "SKILL.md") {
			skill, err := loadSkillFile(path)
			if err != nil {
				diagnostics = append(diagnostics, SkillDiagnostic{
					Type:    "warning",
					Code:    ErrInvalid,
					Message: err.Error(),
					Path:    path,
				})
				continue
			}
			skills = append(skills, skill)
		}
	}

	return skills, diagnostics
}

// loadSkillFile parses a SKILL.md file with optional YAML frontmatter.
func loadSkillFile(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}

	content := string(data)
	skill := Skill{
		FilePath: path,
	}

	// Parse YAML frontmatter (between --- markers)
	if strings.HasPrefix(content, "---") {
		endIdx := strings.Index(content[3:], "---")
		if endIdx >= 0 {
			frontmatter := content[3 : endIdx+3]
			content = content[endIdx+6:]

			// Simple key: value parsing (no YAML dependency)
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					continue
				}
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes
				value = strings.Trim(value, `"'`)

				switch key {
				case "name":
					skill.Name = value
				case "description":
					skill.Description = value
				case "disable-model-invocation":
					skill.DisableModelInvocation = value == "true"
				}
			}
		}
	}

	skill.Content = strings.TrimSpace(content)

	// Default name from filename
	if skill.Name == "" {
		skill.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	return skill, nil
}

// FormatSkillsForSystemPrompt formats skills into a system prompt section.
func FormatSkillsForSystemPrompt(skills []Skill) string {
	var visible []Skill
	for _, s := range skills {
		if !s.DisableModelInvocation {
			visible = append(visible, s)
		}
	}
	if len(visible) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "The following skills provide specialized instructions for specific tasks.")
	lines = append(lines, "Read the full skill file when the task matches its description.")
	lines = append(lines, "When a skill file references a relative path, resolve it against the skill directory.")
	lines = append(lines, "")
	lines = append(lines, "<available_skills>")

	for _, s := range visible {
		lines = append(lines, "  <skill>")
		lines = append(lines, "    <name>"+escapeXML(s.Name)+"</name>")
		lines = append(lines, "    <description>"+escapeXML(s.Description)+"</description>")
		lines = append(lines, "    <location>"+escapeXML(s.FilePath)+"</location>")
		lines = append(lines, "  </skill>")
	}

	lines = append(lines, "</available_skills>")
	return strings.Join(lines, "\n")
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// isIgnored checks if a path should be ignored based on .gitignore patterns.
func isIgnored(path string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// loadIgnorePatterns loads patterns from .gitignore-style files.
func loadIgnorePatterns(dir string) []string {
	var patterns []string
	for _, name := range []string{".gitignore", ".ignore", ".fdignore"} {
		path := filepath.Join(dir, name)
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				patterns = append(patterns, line)
			}
		}
		file.Close()
	}
	return patterns
}
