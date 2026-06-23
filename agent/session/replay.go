package session

func replay(entries []SessionTreeEntry) SessionSnapshot {
	entryByID := make(map[EntryID]SessionTreeEntry)
	for _, e := range entries {
		entryByID[e.ID] = e
	}

	refs := make(map[RefName]EntryID)
	var head Head = Head{Type: "detached"}

	for _, e := range entries {
		switch e.Type {
		case EntrySessionRef:
			refs[e.RefName] = e.RefTargetID
		case EntrySessionCheckout:
			target := e.CheckoutTarget
			if target.Type == "ref" {
				head = Head{Type: "ref", Name: target.Name}
			} else {
				head = Head{Type: "detached", TargetID: target.ID}
			}
		}
	}

	var headTargetID EntryID
	if head.Type == "ref" {
		headTargetID = refs[head.Name]
	} else {
		headTargetID = head.TargetID
	}

	return SessionSnapshot{
		Entries:      entries,
		EntryByID:    entryByID,
		Head:         head,
		HeadTargetID: headTargetID,
		Refs:         refs,
	}
}

func resolveTarget(snapshot SessionSnapshot, target string, useHead bool) EntryID {
	if target == "" {
		if useHead {
			return snapshot.HeadTargetID
		}
		return ""
	}

	if id, ok := snapshot.Refs[target]; ok {
		return id
	}

	if _, ok := snapshot.EntryByID[target]; ok {
		return target
	}

	panic(&SessionError{Code: ErrNotFound, Message: "session target not found: " + target})
}

func semanticPath(snapshot SessionSnapshot, targetID EntryID) []SessionTreeEntry {
	var result []SessionTreeEntry
	currentID := targetID

	for currentID != "" {
		current, ok := snapshot.EntryByID[currentID]
		if !ok {
			panic(&SessionError{Code: ErrNotFound, Message: "session entry not found: " + currentID})
		}

		result = append(result, current)
		currentID = current.ParentID
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

func branchPath(snapshot SessionSnapshot, targetID EntryID) []SessionTreeEntry {
	semantic := semanticPath(snapshot, targetID)
	semanticIDs := make(map[EntryID]bool)
	for _, e := range semantic {
		semanticIDs[e.ID] = true
	}

	var result []SessionTreeEntry
	for _, e := range snapshot.Entries {
		if semanticIDs[e.ID] {
			result = append(result, e)
		} else if e.Type == EntryEvent && e.ParentID != "" && semanticIDs[e.ParentID] {
			result = append(result, e)
		}
	}

	return result
}