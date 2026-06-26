package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	piai "pi-ai-go"
	"pi-ai-go/core"
)

// loadEnv 从 .env 文件加载环境变量（不覆盖已有 env）。
// searchPaths 按顺序查找，找到第一个存在的文件就停止。
func loadEnv(searchPaths ...string) {
	for _, p := range searchPaths {
		file, err := os.Open(p)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// 去掉包裹引号
			if len(value) >= 2 {
				first, last := value[0], value[len(value)-1]
				if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
					value = value[1 : len(value)-1]
				}
			}
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		return
	}
}

// envFirst 按顺序返回第一个非空环境变量。
func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// envBool 解析布尔环境变量。
func envBool(key string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// envInt 解析整数环境变量。
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// resolveReasoning 解析 reasoning 级别。
// 输入：off / minimal / low / medium / high / xhigh（大小写、空格不敏感）；空值默认为 medium。
func resolveReasoning(raw string) core.ThinkingLevel {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", "medium", "med":
		return core.ThinkingMedium
	case "off", "none", "disable", "disabled", "false", "0":
		return ""
	case "minimal", "min":
		return core.ThinkingMinimal
	case "low":
		return core.ThinkingLow
	case "high":
		return core.ThinkingHigh
	case "xhigh", "max":
		return core.ThinkingXHigh
	default:
		fmt.Fprintf(os.Stderr, "[warn] unknown reasoning level %q, fallback to medium\n", raw)
		return core.ThinkingMedium
	}
}

// appConfig 集中管理所有运行时配置。
type appConfig struct {
	APIKey    string
	BaseURL   string
	ModelID   string
	Provider  string
	Reasoning core.ThinkingLevel
	Model     piai.Model
	Verbose   bool
}

// resolveAppConfig 从 env / flag 中组装配置。
// 优先级：flag > LLM_* env > 旧名 (XIAOMI_*, SILICONFLOW_*, etc.) > 默认值。
func resolveAppConfig(verbose bool) appConfig {
	cfg := appConfig{
		APIKey:    envFirst("LLM_API_KEY", "XIAOMI_API_KEY", "SILICONFLOW_API_KEY"),
		BaseURL:   envFirst("LLM_BASE_URL", "XIAOMI_BASE_URL", "SILICONFLOW_BASE_URL"),
		ModelID:   envFirst("LLM_MODEL", "XIAOMI_MODEL", "SILICONFLOW_MODEL"),
		Provider:  envFirst("LLM_PROVIDER", "XIAOMI_PROVIDER"),
		Reasoning: resolveReasoning(envFirst("LLM_REASONING", "REASONING")),
		Verbose:   verbose,
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.siliconflow.cn/v1"
	}
	if cfg.ModelID == "" {
		cfg.ModelID = "Qwen/Qwen2.5-7B-Instruct"
	}
	if cfg.Provider == "" {
		cfg.Provider = string(core.ProviderDeepSeek)
	}

	cfg.Model = piai.Model{
		ID:            cfg.ModelID,
		API:           piai.APIOpenAICompletions,
		Provider:      core.KnownProvider(cfg.Provider),
		BaseURL:       cfg.BaseURL,
		Input:         []piai.Modality{piai.ModalityText},
		ContextWindow: 64000,
		MaxTokens:     4096,
		Cost: piai.Cost{
			Input:  0.14,
			Output: 0.28,
		},
	}
	return cfg
}

func (c appConfig) print() {
	fmt.Fprintf(os.Stderr, "[config] Provider: %s\n", c.Provider)
	fmt.Fprintf(os.Stderr, "[config] Model: %s\n", c.ModelID)
	fmt.Fprintf(os.Stderr, "[config] Base URL: %s\n", c.BaseURL)
	fmt.Fprintf(os.Stderr, "[config] Reasoning: %s\n", c.Reasoning)
}
