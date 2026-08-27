package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ---------- Types ----------

// AIConfig AI 配置
type AIConfig struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	APIKey        string `json:"apiKey"`
	Endpoint      string `json:"endpoint"`
	PromptBase    string `json:"promptBase"`
	MaxWords      int    `json:"maxWords"`
	UseTracing    bool   `json:"useTracing"`
}

// PinyinRequest 拼音请求
type PinyinRequest struct {
	Text string `json:"text"`
}

// PinyinResponse 拼音响应
type PinyinResponse struct {
	Pinyins []string `json:"pinyins"`
}

// GeminiRequest AI 生成请求
type GeminiRequest struct {
	Prompt string `json:"prompt"`
}

// GeminiResponse AI 生成响应
type GeminiResponse struct {
	Phrases []string `json:"phrases"`
}

// ---------- API: Pinyin ----------

// GetPinyin 获取汉字拼音（通过内置查表 + 简单规则）
func (a *App) GetPinyin(text string) []string {
	if text == "" {
		return []string{}
	}

	chars := []rune(text)
	result := make([]string, 0, len(chars))
	for _, ch := range chars {
		s := string(ch)
		if p, ok := pinyinDict[s]; ok {
			result = append(result, p)
		} else {
			result = append(result, s)
		}
	}
	return result
}

// ---------- AI 配置持久化 ----------

var savedConfig AIConfig

// GetAIConfig 获取当前的 AI 配置
func (a *App) GetAIConfig() AIConfig {
	// 如果已有保存的配置则返回
	if savedConfig.APIKey != "" {
		return savedConfig
	}

	cfg := AIConfig{
		Provider:   "gemini",
		Model:      "gemini-2.0-flash",
		PromptBase: "",
		MaxWords:   15,
		UseTracing: true,
	}

	// 尝试从环境变量读取
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		cfg.APIKey = key
	} else if key := os.Getenv("API_KEY"); key != "" {
		cfg.APIKey = key
	} else if data, err := os.ReadFile(".env.local"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "GEMINI_API_KEY=") {
				cfg.APIKey = strings.TrimSpace(strings.TrimPrefix(line, "GEMINI_API_KEY="))
				break
			} else if strings.HasPrefix(line, "API_KEY=") {
				cfg.APIKey = strings.TrimSpace(strings.TrimPrefix(line, "API_KEY="))
				break
			}
		}
	}

	savedConfig = cfg
	return cfg
}

// SaveAIConfig 保存 AI 配置
func (a *App) SaveAIConfig(cfg AIConfig) bool {
	savedConfig = cfg
	// 保存到环境变量（供后续 GenerateContent 使用）
	if cfg.APIKey != "" {
		os.Setenv("GEMINI_API_KEY", cfg.APIKey)
	}
	return true
}

// ---------- API: Gemini AI Content ----------

// GenerateContent 使用 AI 生成练习内容（支持 Gemini / OpenAI 兼容）
func (a *App) GenerateContent(prompt string) []string {
	cfg := savedConfig

	// 自动读取配置
	if cfg.APIKey == "" {
		cfg = a.GetAIConfig()
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		return []string{"请先在 AI 设置中配置 API 密钥"}
	}

	// 构建提示词
	promptBase := cfg.PromptBase
	if promptBase == "" {
		promptBase = `Generate a list of Chinese words or phrases related to the topic: "%s".
Return the result as a simple JSON array of strings.
For each character that is slightly complex or good for practice, occasionally mark it with an asterisk * to indicate it should be a tracing character.
Keep it between %d words/phrases.
Example: ["勤*学*苦*练*", "积极*向上*", "自强*不息*"]`
	}

	maxWords := cfg.MaxWords
	if maxWords <= 0 {
		maxWords = 15
	}

	finalPrompt := fmt.Sprintf(promptBase, prompt, maxWords)

	if cfg.Provider == "openai" || cfg.Provider == "openai-compatible" {
		return a.generateOpenAI(cfg, finalPrompt)
	}
	return a.generateGemini(cfg, finalPrompt)
}

func (a *App) generateGemini(cfg AIConfig, finalPrompt string) []string {
	model := cfg.Model
	if model == "" {
		model = "gemini-2.0-flash"
	}

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": finalPrompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", url.QueryEscape(model), url.QueryEscape(cfg.APIKey))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(apiURL, "application/json", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return []string{fmt.Sprintf("API 请求失败: %v", err)}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return []string{fmt.Sprintf("API 错误 (%d): %s", resp.StatusCode, string(respBody))}
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return []string{fmt.Sprintf("解析响应失败: %v", err)}
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return []string{"AI 未返回有效内容"}
	}

	text := geminiResp.Candidates[0].Content.Parts[0].Text
	return parsePhrases(text)
}

func (a *App) generateOpenAI(cfg AIConfig, finalPrompt string) []string {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimRight(endpoint, "/")

	model := cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": finalPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	bodyBytes, _ := json.Marshal(payload)

	apiURL := endpoint + "/chat/completions"

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("POST", apiURL, strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return []string{fmt.Sprintf("API 请求失败: %v", err)}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return []string{fmt.Sprintf("API 错误 (%d): %s", resp.StatusCode, string(respBody))}
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return []string{fmt.Sprintf("解析响应失败: %v", err)}
	}

	if len(openAIResp.Choices) == 0 {
		return []string{"AI 未返回有效内容"}
	}

	return parsePhrases(openAIResp.Choices[0].Message.Content)
}

// parsePhrases 从 AI 响应文本中解析 JSON 数组
func parsePhrases(text string) []string {
	var phrases []string
	if err := json.Unmarshal([]byte(text), &phrases); err != nil {
		// 尝试从 JSON 对象中提取 {"words": [...]} 格式
		var obj struct {
			Words  []string `json:"words"`
			Phrases []string `json:"phrases"`
			Items   []string `json:"items"`
			Data    []string `json:"data"`
		}
		if e := json.Unmarshal([]byte(text), &obj); e == nil {
			if len(obj.Words) > 0 {
				return obj.Words
			} else if len(obj.Phrases) > 0 {
				return obj.Phrases
			} else if len(obj.Items) > 0 {
				return obj.Items
			} else if len(obj.Data) > 0 {
				return obj.Data
			}
		}

		// 尝试提取 JSON 数组
		start := strings.Index(text, "[")
		end := strings.LastIndex(text, "]")
		if start >= 0 && end > start {
			json.Unmarshal([]byte(text[start:end+1]), &phrases)
		}
	}

	if len(phrases) == 0 {
		return []string{"未能解析 AI 返回的内容"}
	}
	return phrases
}
