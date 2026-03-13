package ui

func defaultReducer(state AppState, action Action) AppState {
	// create a copy of state to avoid mutations
	newState := state

	// TODO: Implement actual state changes based on action types
	//switch a := action.(type) {
	//case SetSearchAction:
	//	newState.SearchQuery = a.Query
	//
	//case SelectRevisionAction:
	//	newState.SelectedRevision = &RevisionSelection{
	//		ObjectID:   a.ObjectID,
	//		RevisionID: a.RevisionID,
	//	}
	//
	//case SetFilterAction:
	//	newState.ActiveFilter = a.Filter
	//
	//case SetViewModeAction:
	//	newState.PreviewMode = a.Mode
	//
	//case ToggleExpandAction:
	//	if newState.ExpandedItems == nil {
	//		newState.ExpandedItems = make(map[string]bool)
	//	}
	//	newState.ExpandedItems[a.ItemID] = !newState.ExpandedItems[a.ItemID]
	//
	//case AddRevisionAction:
	//	res := newState.Resources[a.Revision.ObjectID]
	//	if res == nil {
	//		// Extract resource info from revision
	//		res = &ResourceState{
	//			UID: a.Revision.ObjectID,
	//		}
	//		newState.Resources[a.Revision.ObjectID] = res
	//	}
	//	res.Revisions = append(res.Revisions, a.Revision)
	//	res.LastModified = a.Revision.Time
	//	res.IsActive = time.Since(a.Revision.Time) < 3*time.Second
	//}

	return newState
}

func statesEqual(a, b AppState) bool {
	// TODO: implement actual state comparison logic
	return a == b
}
