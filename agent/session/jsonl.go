package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	core "pi-ai-go/core"
	"sync"
	"time"
)

type JSONLStorage struct {
	mu          sync.Mutex
	path        string
	entries     []SessionTreeEntry
	initialized bool
	queue       *keyedQueue
}

func NewJSONLStorage(path string) (*JSONLStorage, error) {
	return &JSONLStorage{
		path:  path,
		queue: newKeyedQueue(),
	}, nil
}

func (j *JSONLStorage) Append(entries []SessionTreeEntry) error {
	return j.queue.enqueue(j.path, func() error {
		j.mu.Lock()
		defer j.mu.Unlock()

		if !j.initialized {
			if err := j.loadEntries(); err != nil {
				return err
			}
		}

		j.entries = append(j.entries, entries...)

		if len(j.entries) == len(entries) {
			return j.writeFileAtomic(j.encodeAll())
		}

		content := j.appendEncode(entries)
		if content != "" {
			f, err := os.OpenFile(j.path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return &SessionError{Code: ErrStorage, Message: "failed to open file", Path: j.path, Err: err}
			}
			defer f.Close()

			if _, err := f.WriteString(content); err != nil {
				return &SessionError{Code: ErrStorage, Message: "write failed", Err: err}
			}

			if err := f.Sync(); err != nil {
				return &SessionError{Code: ErrStorage, Message: "sync failed", Err: err}
			}
		}

		return nil
	})
}

func (j *JSONLStorage) ReadAll() ([]SessionTreeEntry, error) {
	var result []SessionTreeEntry

	err := j.queue.enqueue(j.path, func() error {
		j.mu.Lock()
		defer j.mu.Unlock()

		if !j.initialized {
			if err := j.loadEntries(); err != nil {
				return err
			}
		}

		result = make([]SessionTreeEntry, len(j.entries))
		copy(result, j.entries)
		return nil
	})

	return result, err
}

func (j *JSONLStorage) Close() error {
	return nil
}

func (j *JSONLStorage) loadEntries() error {
	raw, err := readFileSafe(j.path)
	if err != nil {
		return err
	}

	if raw == "" {
		j.entries = []SessionTreeEntry{}
		j.initialized = true
		return nil
	}

	j.entries = j.decode(raw)
	j.initialized = true
	return nil
}

func readFileSafe(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", &SessionError{Code: ErrStorage, Message: "failed to read file", Path: path, Err: err}
	}
	return string(data), nil
}

func (j *JSONLStorage) writeFileAtomic(content string) error {
	tmpPath := filepath.Join(filepath.Dir(j.path), ".tmp-"+time.Now().Format("20060102150405")+"-"+randomString(8)+".jsonl")

	f, err := os.Create(tmpPath)
	if err != nil {
		return &SessionError{Code: ErrStorage, Message: "failed to create temp file", Err: err}
	}

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return &SessionError{Code: ErrStorage, Message: "write failed", Err: err}
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return &SessionError{Code: ErrStorage, Message: "sync failed", Err: err}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return &SessionError{Code: ErrStorage, Message: "close failed", Err: err}
	}

	if err := os.Rename(tmpPath, j.path); err != nil {
		os.Remove(tmpPath)
		return &SessionError{Code: ErrStorage, Message: "rename failed", Err: err}
	}

	return nil
}

func randomString(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
	}
	return string(result)
}

func (j *JSONLStorage) encodeAll() string {
	var lines []string
	for _, entry := range j.entries {
		raw := entryToRaw(entry)
		if jsonData, err := json.Marshal(raw); err == nil {
			lines = append(lines, string(jsonData))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return joinLines(lines) + "\n"
}

func (j *JSONLStorage) appendEncode(entries []SessionTreeEntry) string {
	var lines []string
	for _, entry := range entries {
		raw := entryToRaw(entry)
		if jsonData, err := json.Marshal(raw); err == nil {
			lines = append(lines, string(jsonData))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n" + joinLines(lines) + "\n"
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for _, line := range lines[1:] {
		result += "\n" + line
	}
	return result
}

func (j *JSONLStorage) decode(raw string) []SessionTreeEntry {
	var entries []SessionTreeEntry
	dec := json.NewDecoder(&countingReader{Reader: bytesReader(raw)})

	for {
		var rawEntry map[string]json.RawMessage
		if err := dec.Decode(&rawEntry); err != nil {
			break
		}

		if len(rawEntry) == 0 {
			continue
		}

		entry, err := rawToEntry(rawEntry)
		if err == nil {
			entries = append(entries, entry)
		}
	}

	return entries
}

type countingReader struct {
	Reader interface {
		Read([]byte) (int, error)
	}
	count int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.count += n
	return n, err
}

func bytesReader(s string) *stringReader {
	return &stringReader{s: s, i: 0}
}

type stringReader struct {
	s string
	i int
}

func (r *stringReader) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, os.ErrClosed
	}
	n := copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}

func entryToRaw(entry SessionTreeEntry) map[string]any {
	m := map[string]any{
		"id":        entry.ID,
		"type":      entry.Type,
		"timestamp": entry.Timestamp,
	}
	if entry.ParentID != "" {
		m["parentId"] = entry.ParentID
	}

	switch entry.Type {
	case EntryMessage:
		if entry.Message != nil {
			m["messageRole"] = getRole(entry.Message)
			m["message"] = entry.Message
		}
	case EntryCustomMessage:
		m["customType"] = entry.CustomType
		m["content"] = entry.Content
		m["display"] = entry.Display
		m["details"] = entry.Details
	case EntryBranchSummary:
		m["summary"] = entry.Summary
		m["fromId"] = entry.FromID
	case EntryCompaction:
		m["compactionSummary"] = entry.CompactionSummary
		m["tokensBefore"] = entry.TokensBefore
		m["firstKeptEntryId"] = entry.FirstKeptEntryID
	case EntryModelChange:
		m["provider"] = entry.Provider
		m["modelId"] = entry.ModelID
	case EntryThinkingChange:
		m["thinkingLevel"] = entry.ThinkingLevel
	case EntrySessionInfo:
		m["sessionId"] = entry.SessionID
		m["description"] = entry.Description
	case EntrySessionRef:
		m["refName"] = entry.RefName
		if entry.RefTargetID != "" {
			m["refTargetId"] = entry.RefTargetID
		}
	case EntrySessionCheckout:
		m["checkoutTarget"] = map[string]any{
			"type": entry.CheckoutTarget.Type,
		}
		if entry.CheckoutTarget.Name != "" {
			m["checkoutTarget"].(map[string]any)["name"] = entry.CheckoutTarget.Name
		}
		if entry.CheckoutTarget.ID != "" {
			m["checkoutTarget"].(map[string]any)["id"] = entry.CheckoutTarget.ID
		}
	case EntryEvent:
		m["eventData"] = entry.EventData
	case EntryLabel:
		m["summary"] = entry.Summary
	}
	return m
}

func getRole(msg core.Message) string {
	switch msg.(type) {
	case core.UserMessage:
		return "user"
	case core.AssistantMessage:
		return "assistant"
	case core.ToolResultMessage:
		return "toolResult"
	default:
		return "unknown"
	}
}

func rawToEntry(raw map[string]json.RawMessage) (SessionTreeEntry, error) {
	var entry SessionTreeEntry

	if v, ok := raw["id"]; ok {
		json.Unmarshal(v, &entry.ID)
	}
	if v, ok := raw["type"]; ok {
		json.Unmarshal(v, &entry.Type)
	}
	if v, ok := raw["timestamp"]; ok {
		json.Unmarshal(v, &entry.Timestamp)
	}
	if v, ok := raw["parentId"]; ok {
		json.Unmarshal(v, &entry.ParentID)
	}

	switch entry.Type {
	case EntryMessage:
		var role string
		if v, ok := raw["messageRole"]; ok {
			json.Unmarshal(v, &role)
		}
		msgRaw, hasMsg := raw["message"]
		if hasMsg {
			switch role {
			case "user":
				var m core.UserMessage
				json.Unmarshal(msgRaw, &m)
				entry.Message = m
			case "assistant":
				var m core.AssistantMessage
				json.Unmarshal(msgRaw, &m)
				entry.Message = m
			case "toolResult":
				var m core.ToolResultMessage
				json.Unmarshal(msgRaw, &m)
				entry.Message = m
			}
		}
	case EntryCustomMessage:
		if v, ok := raw["customType"]; ok {
			json.Unmarshal(v, &entry.CustomType)
		}
		if v, ok := raw["content"]; ok {
			json.Unmarshal(v, &entry.Content)
		}
		if v, ok := raw["display"]; ok {
			json.Unmarshal(v, &entry.Display)
		}
		if v, ok := raw["details"]; ok {
			json.Unmarshal(v, &entry.Details)
		}
	case EntryBranchSummary:
		if v, ok := raw["summary"]; ok {
			json.Unmarshal(v, &entry.Summary)
		}
		if v, ok := raw["fromId"]; ok {
			json.Unmarshal(v, &entry.FromID)
		}
	case EntryCompaction:
		if v, ok := raw["compactionSummary"]; ok {
			json.Unmarshal(v, &entry.CompactionSummary)
		}
		if v, ok := raw["tokensBefore"]; ok {
			json.Unmarshal(v, &entry.TokensBefore)
		}
		if v, ok := raw["firstKeptEntryId"]; ok {
			json.Unmarshal(v, &entry.FirstKeptEntryID)
		}
	case EntryModelChange:
		if v, ok := raw["provider"]; ok {
			json.Unmarshal(v, &entry.Provider)
		}
		if v, ok := raw["modelId"]; ok {
			json.Unmarshal(v, &entry.ModelID)
		}
	case EntryThinkingChange:
		if v, ok := raw["thinkingLevel"]; ok {
			json.Unmarshal(v, &entry.ThinkingLevel)
		}
	case EntrySessionInfo:
		if v, ok := raw["sessionId"]; ok {
			json.Unmarshal(v, &entry.SessionID)
		}
		if v, ok := raw["description"]; ok {
			json.Unmarshal(v, &entry.Description)
		}
	case EntrySessionRef:
		if v, ok := raw["refName"]; ok {
			json.Unmarshal(v, &entry.RefName)
		}
		if v, ok := raw["refTargetId"]; ok {
			json.Unmarshal(v, &entry.RefTargetID)
		}
	case EntrySessionCheckout:
		if v, ok := raw["checkoutTarget"]; ok {
			var target struct {
				Type string  `json:"type"`
				Name RefName `json:"name,omitempty"`
				ID   EntryID `json:"id,omitempty"`
			}
			json.Unmarshal(v, &target)
			entry.CheckoutTarget.Type = target.Type
			entry.CheckoutTarget.Name = target.Name
			entry.CheckoutTarget.ID = target.ID
		}
	case EntryEvent:
		if v, ok := raw["eventData"]; ok {
			var data any
			json.Unmarshal(v, &data)
			entry.EventData = data
		}
	case EntryLabel:
		if v, ok := raw["summary"]; ok {
			json.Unmarshal(v, &entry.Summary)
		}
	}

	return entry, nil
}
