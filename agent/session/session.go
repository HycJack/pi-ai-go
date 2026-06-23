package session

import (
	"sync"
	"time"

	core "pi-ai-go/core"
)

type Session struct {
	mu                   sync.RWMutex
	storage              SessionStorage
	entries              []SessionTreeEntry
	id                   string
	queue                *mutationQueue
	initialized          bool
	defaultRef           RefName
	branchChangeHandlers []BranchChangeHandler
}

type NewSessionOptions struct {
	Storage    SessionStorage
	DefaultRef RefName
}

func NewSession(opts NewSessionOptions) (*Session, error) {
	s := &Session{
		storage:    opts.Storage,
		queue:      newMutationQueue(),
		defaultRef: opts.DefaultRef,
	}

	entries, err := opts.Storage.ReadAll()
	if err != nil {
		return nil, &SessionError{Code: ErrStorage, Message: "failed to load session", Err: err}
	}
	s.entries = entries

	for _, e := range entries {
		if e.Type == EntrySessionInfo && e.SessionID != "" {
			s.id = e.SessionID
			break
		}
	}

	if opts.DefaultRef != "" && len(entries) == 0 {
		if err := s.initializeDefaultRef(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

func (s *Session) initializeDefaultRef() error {
	if err := validateRef(s.defaultRef); err != nil {
		return err
	}

	refEntry := SessionTreeEntry{
		ID:        GenerateID(),
		Type:      EntrySessionRef,
		Timestamp: time.Now(),
		RefName:   s.defaultRef,
	}

	checkoutEntry := SessionTreeEntry{
		ID:        GenerateID(),
		Type:      EntrySessionCheckout,
		Timestamp: time.Now(),
	}
	checkoutEntry.CheckoutTarget.Type = "ref"
	checkoutEntry.CheckoutTarget.Name = s.defaultRef

	return s.Append(refEntry, checkoutEntry)
}

func (s *Session) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

func (s *Session) Append(entries ...SessionTreeEntry) error {
	return s.queue.enqueue(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.storage != nil {
			if err := s.storage.Append(entries); err != nil {
				return &SessionError{Code: ErrStorage, Message: "failed to append entries", Err: err}
			}
		}

		for _, e := range entries {
			if e.Type == EntrySessionInfo && e.SessionID != "" {
				s.id = e.SessionID
			}
		}

		s.entries = append(s.entries, entries...)
		return nil
	})
}

func (s *Session) Entries() []SessionTreeEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SessionTreeEntry, len(s.entries))
	copy(result, s.entries)
	return result
}

func (s *Session) BuildContext() SessionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return BuildSessionContext(s.entries)
}

func BuildSessionContext(entries []SessionTreeEntry) SessionContext {
	var thinkingLevel string
	var model *SessionModel
	compactionIdx := -1

	for i, entry := range entries {
		switch entry.Type {
		case EntryThinkingChange:
			thinkingLevel = entry.ThinkingLevel
		case EntryModelChange:
			model = &SessionModel{Provider: entry.Provider, ModelID: entry.ModelID}
		case EntryCompaction:
			compactionIdx = i
		}
	}

	var messages []core.Message

	if compactionIdx >= 0 {
		compaction := entries[compactionIdx]
		messages = append(messages, core.UserMessage{
			Role:      "user",
			Content:   compactionPrefix + compaction.CompactionSummary + compactionSuffix,
			Timestamp: compaction.Timestamp,
		})

		foundFirstKept := false
		for i := 0; i < compactionIdx; i++ {
			if entries[i].ID == compaction.FirstKeptEntryID {
				foundFirstKept = true
			}
			if foundFirstKept {
				if msg := entryToMessage(entries[i]); msg != nil {
					messages = append(messages, msg)
				}
			}
		}
		for i := compactionIdx + 1; i < len(entries); i++ {
			if msg := entryToMessage(entries[i]); msg != nil {
				messages = append(messages, msg)
			}
		}
	} else {
		for _, entry := range entries {
			if msg := entryToMessage(entry); msg != nil {
				messages = append(messages, msg)
			}
		}
	}

	return SessionContext{
		Messages:      messages,
		ThinkingLevel: thinkingLevel,
		Model:         model,
	}
}

func entryToMessage(entry SessionTreeEntry) core.Message {
	switch entry.Type {
	case EntryMessage:
		return entry.Message
	case EntryCustomMessage:
		return core.UserMessage{
			Role:      "user",
			Content:   "[" + entry.CustomType + "] " + stringifyAny(entry.Content),
			Timestamp: entry.Timestamp,
		}
	case EntryBranchSummary:
		return core.UserMessage{
			Role:      "user",
			Content:   branchPrefix + entry.Summary + branchSuffix,
			Timestamp: entry.Timestamp,
		}
	default:
		return nil
	}
}

func stringifyAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storage != nil {
		return s.storage.Close()
	}
	return nil
}

func (s *Session) Read() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return replay(s.entries)
}

func (s *Session) Head() Head {
	return s.Read().Head
}

func (s *Session) Refs() map[RefName]EntryID {
	return s.Read().Refs
}

func (s *Session) Path(target ...string) []SessionTreeEntry {
	snapshot := s.Read()
	var targetID EntryID
	if len(target) > 0 && target[0] != "" {
		targetID = resolveTarget(snapshot, target[0], false)
	} else {
		targetID = snapshot.HeadTargetID
	}
	return branchPath(snapshot, targetID)
}

func (s *Session) Checkout(target string) error {
	return s.queue.enqueue(func() error {
		snapshot := s.Read()
		if err := s.assertIdle(snapshot); err != nil {
			return err
		}

		var checkoutEntry SessionTreeEntry
		checkoutEntry.ID = GenerateID()
		checkoutEntry.Type = EntrySessionCheckout
		checkoutEntry.Timestamp = time.Now()

		if target == "" {
			checkoutEntry.CheckoutTarget.Type = "empty"
		} else if _, ok := snapshot.Refs[target]; ok {
			checkoutEntry.CheckoutTarget.Type = "ref"
			checkoutEntry.CheckoutTarget.Name = target
		} else if _, ok := snapshot.EntryByID[target]; ok {
			checkoutEntry.CheckoutTarget.Type = "id"
			checkoutEntry.CheckoutTarget.ID = target
		} else {
			return &SessionError{Code: ErrNotFound, Message: "session target not found: " + target}
		}

		if err := s.Append(checkoutEntry); err != nil {
			return err
		}

		newSnapshot := s.Read()
		s.notifyBranchChange(BranchChangePayload{
			Type:     "checkout",
			Ref:      newSnapshot.Head.Name,
			TargetID: newSnapshot.HeadTargetID,
		})

		return nil
	})
}

func (s *Session) Fork(name RefName, opts *SessionForkOptions) error {
	return s.queue.enqueue(func() error {
		if err := validateRef(name); err != nil {
			return err
		}

		snapshot := s.Read()
		if err := s.assertIdle(snapshot); err != nil {
			return err
		}

		var fromID EntryID
		if opts != nil && opts.From != "" {
			fromID = resolveTarget(snapshot, opts.From, false)
		} else {
			fromID = snapshot.HeadTargetID
		}

		refEntry := SessionTreeEntry{
			ID:          GenerateID(),
			Type:        EntrySessionRef,
			Timestamp:   time.Now(),
			RefName:     name,
			RefTargetID: fromID,
		}

		entries := []SessionTreeEntry{refEntry}

		if opts == nil || opts.Checkout {
			checkoutEntry := SessionTreeEntry{
				ID:        GenerateID(),
				Type:      EntrySessionCheckout,
				Timestamp: time.Now(),
			}
			checkoutEntry.CheckoutTarget.Type = "ref"
			checkoutEntry.CheckoutTarget.Name = name
			entries = append(entries, checkoutEntry)
		}

		if err := s.Append(entries...); err != nil {
			return err
		}

		if opts == nil || opts.Checkout {
			newSnapshot := s.Read()
			s.notifyBranchChange(BranchChangePayload{
				Type:     "fork",
				Ref:      name,
				TargetID: newSnapshot.HeadTargetID,
			})
		}

		return nil
	})
}

func (s *Session) Rebase(name RefName, onto string) (*RebaseResult, error) {
	var result *RebaseResult

	err := s.queue.enqueue(func() error {
		snapshot := s.Read()
		if err := s.assertIdle(snapshot); err != nil {
			return err
		}

		oldHeadID, ok := snapshot.Refs[name]
		if !ok {
			return &SessionError{Code: ErrNotFound, Message: "session ref not found: " + name}
		}

		newBaseID := resolveTarget(snapshot, onto, true)

		source := semanticPath(snapshot, oldHeadID)
		targetPath := semanticPath(snapshot, newBaseID)
		targetIDs := make(map[EntryID]bool)
		for _, e := range targetPath {
			targetIDs[e.ID] = true
		}

		var ancestor *SessionTreeEntry
		for i := len(source) - 1; i >= 0; i-- {
			if targetIDs[source[i].ID] {
				ancestor = &source[i]
				break
			}
		}

		var copied []SessionTreeEntry
		startIdx := 0
		if ancestor != nil {
			for i, e := range source {
				if e.ID == ancestor.ID {
					startIdx = i + 1
					break
				}
			}
		}
		copied = source[startIdx:]

		var parentId EntryID = newBaseID
		mapping := make([]struct {
			NewID EntryID `json:"newId"`
			OldID EntryID `json:"oldId"`
		}, 0, len(copied))

		var newEntries []SessionTreeEntry
		for _, entry := range copied {
			newEntry := SessionTreeEntry{
				ID:                GenerateID(),
				Type:              entry.Type,
				Timestamp:         time.Now(),
				ParentID:          parentId,
				Message:           entry.Message,
				CustomType:        entry.CustomType,
				Content:           entry.Content,
				Display:           entry.Display,
				Details:           entry.Details,
				Summary:           entry.Summary,
				FromID:            entry.FromID,
				CompactionSummary: entry.CompactionSummary,
				TokensBefore:      entry.TokensBefore,
				FirstKeptEntryID:  entry.FirstKeptEntryID,
				Provider:          entry.Provider,
				ModelID:           entry.ModelID,
				ThinkingLevel:     entry.ThinkingLevel,
				SessionID:         entry.SessionID,
				Description:       entry.Description,
			}

			mapping = append(mapping, struct {
				NewID EntryID `json:"newId"`
				OldID EntryID `json:"oldId"`
			}{NewID: newEntry.ID, OldID: entry.ID})

			parentId = newEntry.ID
			newEntries = append(newEntries, newEntry)
		}

		refEntry := SessionTreeEntry{
			ID:          GenerateID(),
			Type:        EntrySessionRef,
			Timestamp:   time.Now(),
			RefName:     name,
			RefTargetID: parentId,
		}
		newEntries = append(newEntries, refEntry)

		if err := s.Append(newEntries...); err != nil {
			return err
		}

		newSnapshot := s.Read()
		if newSnapshot.Head.Type == "ref" && newSnapshot.Head.Name == name {
			s.notifyBranchChange(BranchChangePayload{
				Type:     "rebase",
				Ref:      name,
				TargetID: newSnapshot.HeadTargetID,
			})
		}

		var oldBaseID EntryID
		if ancestor != nil {
			oldBaseID = ancestor.ID
		}

		result = &RebaseResult{
			Entries:   mapping,
			Name:      name,
			NewBaseID: newBaseID,
			NewHeadID: parentId,
			OldBaseID: oldBaseID,
			OldHeadID: oldHeadID,
		}

		return nil
	})

	return result, err
}

func (s *Session) Clone(opts SessionCloneOptions) (*Session, error) {
	var cloned *Session
	var cloneErr error

	err := s.queue.enqueue(func() error {
		snapshot := s.Read()
		if err := s.assertIdle(snapshot); err != nil {
			return err
		}

		var selectedRefs []RefName
		if opts.Refs == "all" {
			for refName := range snapshot.Refs {
				selectedRefs = append(selectedRefs, refName)
			}
		} else if opts.Refs != "" && opts.Refs != "active" {
			for _, refName := range []string{opts.Refs} {
				if _, ok := snapshot.Refs[refName]; !ok {
					return &SessionError{Code: ErrNotFound, Message: "session ref not found: " + refName}
				}
				selectedRefs = append(selectedRefs, refName)
			}
		} else if snapshot.Head.Type == "ref" {
			selectedRefs = append(selectedRefs, snapshot.Head.Name)
		}

		var fromID EntryID
		if opts.From != "" {
			fromID = resolveTarget(snapshot, opts.From, false)
		} else {
			fromID = snapshot.HeadTargetID
		}

		targetIDs := []EntryID{fromID}
		for _, refName := range selectedRefs {
			targetIDs = append(targetIDs, snapshot.Refs[refName])
		}

		semanticIDs := make(map[EntryID]bool)
		for _, tid := range targetIDs {
			for _, e := range semanticPath(snapshot, tid) {
				semanticIDs[e.ID] = true
			}
		}

		var copied []SessionTreeEntry
		for _, e := range snapshot.Entries {
			if semanticIDs[e.ID] ||
				(e.Type == EntryEvent && e.ParentID != "" && semanticIDs[e.ParentID]) {
				copied = append(copied, e)
			}
		}

		var controls []SessionTreeEntry
		for _, refName := range selectedRefs {
			controls = append(controls, SessionTreeEntry{
				ID:          GenerateID(),
				Type:        EntrySessionRef,
				Timestamp:   time.Now(),
				RefName:     refName,
				RefTargetID: snapshot.Refs[refName],
			})
		}

		if err := opts.SessionStorage.Append(append(copied, controls...)); err != nil {
			return &SessionError{Code: ErrStorage, Message: "failed to clone session", Err: err}
		}

		var checkoutTarget string
		if opts.Checkout != "" {
			checkoutTarget = opts.Checkout
		} else if snapshot.Head.Type == "ref" {
			checkoutTarget = snapshot.Head.Name
		} else {
			checkoutTarget = fromID
		}

		clonedOpts := NewSessionOptions{
			Storage:    opts.SessionStorage,
			DefaultRef: "",
		}
		var newCloned *Session
		newCloned, cloneErr = NewSession(clonedOpts)
		if cloneErr != nil {
			return cloneErr
		}
		cloned = newCloned

		return cloned.Checkout(checkoutTarget)
	})

	if err != nil {
		return nil, err
	}
	return cloned, cloneErr
}

func (s *Session) assertIdle(snapshot SessionSnapshot) error {
	active := make(map[string]bool)
	for _, entry := range snapshot.Entries {
		if entry.Type != EntryEvent {
			continue
		}

		if entry.EventData == nil {
			continue
		}

		data, ok := entry.EventData.(map[string]any)
		if !ok {
			continue
		}

		eventType, _ := data["type"].(string)
		turnID, _ := data["turnId"].(string)

		if eventType == "turn.start" && turnID != "" {
			active[turnID] = true
		} else if (eventType == "turn.done" || eventType == "turn.failed" || eventType == "turn.aborted") && turnID != "" {
			delete(active, turnID)
		}
	}

	if len(active) > 0 {
		return &SessionError{Code: ErrBusy, Message: "session has an active agent turn"}
	}

	return nil
}

func (s *Session) notifyBranchChange(payload BranchChangePayload) {
	for _, handler := range s.branchChangeHandlers {
		go handler(payload)
	}
}

func (s *Session) AddBranchChangeHandler(handler BranchChangeHandler) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := len(s.branchChangeHandlers)
	s.branchChangeHandlers = append(s.branchChangeHandlers, handler)

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if idx < len(s.branchChangeHandlers) {
			s.branchChangeHandlers = append(s.branchChangeHandlers[:idx], s.branchChangeHandlers[idx+1:]...)
		}
	}
}

type MemoryStorage struct {
	mu      sync.RWMutex
	entries []SessionTreeEntry
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{}
}

func (m *MemoryStorage) Append(entries []SessionTreeEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entries...)
	return nil
}

func (m *MemoryStorage) ReadAll() ([]SessionTreeEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]SessionTreeEntry, len(m.entries))
	copy(result, m.entries)
	return result, nil
}

func (m *MemoryStorage) Close() error { return nil }

func GenerateID() string {
	return time.Now().Format("20060102150405.000000000")
}
