package rollup_test

import (
	"testing"

	"github.com/enricojoe/dailychecker/internal/rollup"
)

// stateOf looks up the resulting state for id in the Apply output.
func stateOf(t *testing.T, changes []rollup.NodeState, id string) (string, bool) {
	t.Helper()
	for _, c := range changes {
		if c.ID == id {
			return c.State, true
		}
	}
	return "", false
}

func TestApply(t *testing.T) {
	const (
		parentID = "parent-1"
		child1   = "child-1"
		child2   = "child-2"
		child3   = "child-3"

		// Short aliases for the state constants.
		P = rollup.StatePending
		Q = rollup.StatePartial
		D = rollup.StateDone
	)

	nd := func(id, state string) rollup.NodeState {
		return rollup.NodeState{ID: id, State: state}
	}

	tests := []struct {
		name       string
		parent     rollup.NodeState
		children   []rollup.NodeState
		changedID  string
		newState   string
		wantLen    int
		wantStates map[string]string // id → expected resulting state
	}{
		// ── Rule 5: leaf (no children) ──────────────────────────────────────
		{
			name:       "leaf: set parent done — no propagation",
			parent:     nd(parentID, P),
			children:   nil,
			changedID:  parentID, newState: D,
			wantLen:    1,
			wantStates: map[string]string{parentID: D},
		},
		{
			name:       "leaf: set parent partial — no propagation",
			parent:     nd(parentID, P),
			children:   nil,
			changedID:  parentID, newState: Q,
			wantLen:    1,
			wantStates: map[string]string{parentID: Q},
		},
		{
			name:       "leaf: set parent pending — no propagation",
			parent:     nd(parentID, D),
			children:   nil,
			changedID:  parentID, newState: P,
			wantLen:    1,
			wantStates: map[string]string{parentID: P},
		},

		// ── Rule 1: all children done → parent auto done ─────────────────────
		{
			name:   "all children done → parent done (two children)",
			parent: nd(parentID, P),
			children: []rollup.NodeState{nd(child1, D), nd(child2, P)},
			changedID: child2, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child2: D, parentID: D},
		},
		{
			name:     "single child set done → parent done",
			parent:   nd(parentID, P),
			children: []rollup.NodeState{nd(child1, P)},
			changedID: child1, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child1: D, parentID: D},
		},
		{
			name:   "all three children done → parent done",
			parent: nd(parentID, Q),
			children: []rollup.NodeState{nd(child1, D), nd(child2, D), nd(child3, P)},
			changedID: child3, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child3: D, parentID: D},
		},

		// ── Rule 2: parent set done → cascade all children to done ───────────
		{
			name:   "parent set done → all pending children become done",
			parent: nd(parentID, P),
			children: []rollup.NodeState{nd(child1, P), nd(child2, P)},
			changedID: parentID, newState: D,
			wantLen:    3,
			wantStates: map[string]string{parentID: D, child1: D, child2: D},
		},
		{
			name:   "parent set done with mixed children → all become done",
			parent: nd(parentID, Q),
			children: []rollup.NodeState{nd(child1, D), nd(child2, P)},
			changedID: parentID, newState: D,
			wantLen:    3,
			wantStates: map[string]string{parentID: D, child1: D, child2: D},
		},

		// ── Rule 3: child unchecked → recompute parent ───────────────────────
		{
			name:   "child done→pending, other child still done → parent partial",
			parent: nd(parentID, D),
			children: []rollup.NodeState{nd(child1, D), nd(child2, D)},
			changedID: child1, newState: P,
			wantLen:    2,
			wantStates: map[string]string{child1: P, parentID: Q},
		},
		{
			name:   "child done→pending, no other child done → parent pending",
			parent: nd(parentID, D),
			children: []rollup.NodeState{nd(child1, D), nd(child2, P)},
			changedID: child1, newState: P,
			wantLen:    2,
			wantStates: map[string]string{child1: P, parentID: P},
		},
		{
			name:     "single child unchecked (done→pending) → parent pending",
			parent:   nd(parentID, D),
			children: []rollup.NodeState{nd(child1, D)},
			changedID: child1, newState: P,
			wantLen:    2,
			wantStates: map[string]string{child1: P, parentID: P},
		},
		{
			name:   "child done→pending then pending→done again → parent done (re-check, all done)",
			parent: nd(parentID, Q),
			children: []rollup.NodeState{nd(child1, D), nd(child2, P)},
			changedID: child2, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child2: D, parentID: D},
		},

		// ── Rule 4: manual partial on any item ───────────────────────────────
		{
			name:   "parent set partial → children unchanged, no cascade",
			parent: nd(parentID, D),
			children: []rollup.NodeState{nd(child1, D), nd(child2, D)},
			changedID: parentID, newState: Q,
			wantLen:    1,
			wantStates: map[string]string{parentID: Q},
		},
		{
			name:   "parent set pending → children unchanged, no cascade",
			parent: nd(parentID, D),
			children: []rollup.NodeState{nd(child1, D), nd(child2, D)},
			changedID: parentID, newState: P,
			wantLen:    1,
			wantStates: map[string]string{parentID: P},
		},
		{
			name:   "child set partial (from done), other child still done → parent partial",
			parent: nd(parentID, D),
			children: []rollup.NodeState{nd(child1, D), nd(child2, D)},
			changedID: child1, newState: Q,
			wantLen:    2,
			wantStates: map[string]string{child1: Q, parentID: Q},
		},
		{
			name:   "child set partial (from done), no other child done → parent pending",
			parent: nd(parentID, D),
			children: []rollup.NodeState{nd(child1, D), nd(child2, P)},
			changedID: child1, newState: Q,
			wantLen:    2,
			wantStates: map[string]string{child1: Q, parentID: P},
		},

		// ── Re-check: child not-done→done ────────────────────────────────────
		{
			name:   "child pending→done, another still pending → parent partial",
			parent: nd(parentID, P),
			children: []rollup.NodeState{nd(child1, P), nd(child2, P)},
			changedID: child1, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child1: D, parentID: Q},
		},
		{
			name:   "child partial→done, all now done → parent done",
			parent: nd(parentID, Q),
			children: []rollup.NodeState{nd(child1, D), nd(child2, Q)},
			changedID: child2, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child2: D, parentID: D},
		},

		// ── Mixed states ─────────────────────────────────────────────────────
		{
			name: "three children: 1 done 1 partial 1 pending → set pending to done → 2 done, parent partial",
			parent: nd(parentID, Q),
			children: []rollup.NodeState{nd(child1, D), nd(child2, Q), nd(child3, P)},
			changedID: child3, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child3: D, parentID: Q},
		},
		{
			name: "three children: 1 done 1 pending 1 pending → set one pending to done → partial",
			parent: nd(parentID, Q),
			children: []rollup.NodeState{nd(child1, D), nd(child2, P), nd(child3, P)},
			changedID: child3, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child3: D, parentID: Q},
		},

		// ── Manual-override precedence: prior cascade then child unchecked ────
		{
			name:   "post-cascade: child unchecked (one sibling still done) → parent partial",
			parent: nd(parentID, D), // was cascade-set done by rule 2
			children: []rollup.NodeState{nd(child1, D), nd(child2, D)},
			changedID: child1, newState: P,
			wantLen:    2,
			wantStates: map[string]string{child1: P, parentID: Q},
		},
		{
			name:   "post-cascade: both children unchecked → parent pending",
			// Simulates unchecking child2 after child1 was already unchecked.
			parent: nd(parentID, Q), // recomputed to partial by prior engine call
			children: []rollup.NodeState{nd(child1, P), nd(child2, D)},
			changedID: child2, newState: P,
			wantLen:    2,
			wantStates: map[string]string{child2: P, parentID: P},
		},
		{
			name:   "prior manual-partial parent overridden when all children done",
			parent: nd(parentID, Q), // was manually set partial
			children: []rollup.NodeState{nd(child1, D), nd(child2, P)},
			changedID: child2, newState: D,
			wantLen:    2,
			wantStates: map[string]string{child2: D, parentID: D},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changes := rollup.Apply(tc.parent, tc.children, tc.changedID, tc.newState)

			if len(changes) != tc.wantLen {
				t.Errorf("Apply() returned %d changes, want %d; got %v", len(changes), tc.wantLen, changes)
			}

			for id, wantState := range tc.wantStates {
				gotState, found := stateOf(t, changes, id)
				if !found {
					t.Errorf("expected change for %q not present in result %v", id, changes)
					continue
				}
				if gotState != wantState {
					t.Errorf("state of %q = %q, want %q", id, gotState, wantState)
				}
			}
		})
	}
}
