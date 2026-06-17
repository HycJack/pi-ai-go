package autolearn

import (
	"context"
	"path/filepath"
	"testing"

	"examples/agent-with-skills/memory"
)

func TestExtractFromUserInput(t *testing.T) {
	tests := []struct {
		input   string
		wantLen int
		wantKey string
		wantVal string
	}{
		{
			input:   "请记住：user.name=小明",
			wantLen: 1,
			wantKey: "user.name",
			wantVal: "小明",
		},
		{
			input:   "记住：preference.language=zh-CN",
			wantLen: 1,
			wantKey: "preference.language",
			wantVal: "zh-CN",
		},
		{
			input:   "[remember:project.name=pi-ai-go]",
			wantLen: 1,
			wantKey: "project.name",
			wantVal: "pi-ai-go",
		},
		{
			input:   "[memorize:user.email=test@example.com]",
			wantLen: 1,
			wantKey: "user.email",
			wantVal: "test@example.com",
		},
		{
			input:   "普通对话，没有记忆标记",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			triggers := ExtractFromUserInput(tt.input)
			if len(triggers) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(triggers), tt.wantLen)
			}
			if tt.wantLen > 0 && len(triggers) > 0 {
				if triggers[0].Key != tt.wantKey {
					t.Errorf("key = %q, want %q", triggers[0].Key, tt.wantKey)
				}
				if triggers[0].Value != tt.wantVal {
					t.Errorf("value = %q, want %q", triggers[0].Value, tt.wantVal)
				}
				if triggers[0].Source != SourceUserInput {
					t.Errorf("source = %s, want %s", triggers[0].Source, SourceUserInput)
				}
			}
		})
	}
}

func TestExtractFromToolResult(t *testing.T) {
	tests := []struct {
		input   string
		wantLen int
		wantKey string
		wantVal string
	}{
		{
			input:   "REMEMBER:user.name=小明",
			wantLen: 1,
			wantKey: "user.name",
			wantVal: "小明",
		},
		{
			input:   "请记住：config.key=value",
			wantLen: 1,
			wantKey: "config.key",
			wantVal: "value",
		},
		{
			input:   "tool output without marker",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			triggers := ExtractFromToolResult(tt.input)
			if len(triggers) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(triggers), tt.wantLen)
			}
			if tt.wantLen > 0 && len(triggers) > 0 {
				if triggers[0].Key != tt.wantKey {
					t.Errorf("key = %q, want %q", triggers[0].Key, tt.wantKey)
				}
			}
		})
	}
}

func TestAutoLearnerApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	mem, err := memory.New(path)
	if err != nil {
		t.Fatal(err)
	}

	al := New(mem, DefaultSettings())

	// Process user input
	count := al.ProcessUserInput("请记住：user.name=小明")
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Verify stored
	if v, _ := mem.Get("user.name"); v != "小明" {
		t.Errorf("mem.Get = %q, want 小明", v)
	}

	// Process more
	al.ProcessUserInput("[remember:preference.language=zh-CN]")
	al.ProcessToolResult("REMEMBER:project.name=pi-ai-go")

	if mem.Size() != 3 {
		t.Errorf("mem.Size = %d, want 3", mem.Size())
	}
}

func TestAutoLearnerNilMemory(t *testing.T) {
	al := New(nil, DefaultSettings())
	// 不应 panic
	count := al.ProcessUserInput("请记住：user.name=小明")
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestAutoLearnerCategorization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	mem, _ := memory.New(path)
	al := New(mem, DefaultSettings())

	al.ProcessUserInput("请记住：user.name=小明")
	al.ProcessToolResult("REMEMBER:tool.foo=bar")

	// 应该按来源分类
	users := mem.ListByCategory(string(SourceUserInput))
	if len(users) != 1 {
		t.Errorf("ListByCategory(user) = %d, want 1", len(users))
	}
	tools := mem.ListByCategory(string(SourceToolResult))
	if len(tools) != 1 {
		t.Errorf("ListByCategory(tool) = %d, want 1", len(tools))
	}
}

func TestParseExtractionResult(t *testing.T) {
	tests := []struct {
		input   string
		wantLen int
	}{
		{
			input:   "user.name=小明\npreference.theme=dark",
			wantLen: 2,
		},
		{
			input:   "NONE",
			wantLen: 0,
		},
		{
			input:   "",
			wantLen: 0,
		},
		{
			input:   "user.name=小明\n# 注释行\nkey=val",
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			triggers := parseExtractionResult(tt.input, SourceLLMExtract)
			if len(triggers) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(triggers), tt.wantLen)
			}
		})
	}
}

func TestMaybeExtractDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	mem, _ := memory.New(path)

	// Default settings: AutoLearn=false
	al := New(mem, DefaultSettings())

	extractor := &LLMSimpleExtractor{
		SummarizeFunc: func(ctx context.Context, prompt string) (string, error) {
			return "user.name=test", nil
		},
	}

	// AutoLearn = false，不应触发
	result := al.MaybeExtract(context.Background(), nil, extractor)
	if result > 0 {
		t.Errorf("MaybeExtract should return 0 when AutoLearn=false, got %d", result)
	}
}

func TestMaybeExtractEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	mem, _ := memory.New(path)

	settings := DefaultSettings()
	settings.AutoLearn = true
	settings.ExtractEveryN = 2
	al := New(mem, settings)

	extractor := &LLMSimpleExtractor{
		SummarizeFunc: func(ctx context.Context, prompt string) (string, error) {
			return "user.name=test", nil
		},
	}

	// 第 1 次不触发
	al.MaybeExtract(context.Background(), nil, extractor)
	if mem.Size() != 0 {
		t.Errorf("after 1st call, mem.Size = %d, want 0", mem.Size())
	}

	// 第 2 次触发
	al.MaybeExtract(context.Background(), nil, extractor)
	if mem.Size() != 1 {
		t.Errorf("after 2nd call, mem.Size = %d, want 1", mem.Size())
	}
}

// TestSettingsAccessor 验证 Settings() 返回正确的配置。
func TestSettingsAccessor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	mem, _ := memory.New(path)

	settings := Settings{
		AutoLearn:     true,
		ExtractEveryN: 7,
		MinConfidence: 0.9,
	}
	al := New(mem, settings)

	got := al.Settings()
	if got.AutoLearn != true {
		t.Error("Settings().AutoLearn mismatch")
	}
	if got.ExtractEveryN != 7 {
		t.Errorf("Settings().ExtractEveryN = %d, want 7", got.ExtractEveryN)
	}
	if got.MinConfidence != 0.9 {
		t.Errorf("Settings().MinConfidence = %f, want 0.9", got.MinConfidence)
	}
}

// TestExtractParseMultiple 验证多行 KEY=VALUE 解析。
func TestExtractParseMultiple(t *testing.T) {
	input := `user.name=小明
preference.theme=dark
# 注释
project.name=pi-ai-go`
	got := parseExtractionResult(input, SourceLLMExtract)
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
	if got[0].Source != SourceLLMExtract {
		t.Errorf("got[0].Source = %s, want %s", got[0].Source, SourceLLMExtract)
	}
}
