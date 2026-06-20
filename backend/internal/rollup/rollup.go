// Package rollup implements the parent/child state-propagation rules for
// activity-occurrence groups. It is a pure, in-memory computation with no
// database or HTTP dependencies; infrastructure layers call Apply, then persist
// the returned change-set.
//
// # State values
//
// Each occurrence carries one of three states: "pending", "partial", or "done".
// Use the package constants StatePending, StatePartial, and StateDone.
//
// # Rollup rules
//
// The engine operates on a "group" consisting of one parent occurrence and zero
// or more child occurrences (all for the same calendar date).
//
//  1. All-children-done (auto-rollup): when every child transitions to "done",
//     the parent is automatically set to "done".
//
//  2. Parent-done cascade: when the parent is explicitly set to "done", all
//     child occurrences are also set to "done".
//
//  3. Child unchecked (recompute): when a child moves away from "done" (to
//     "pending" or "partial"), the parent state is recomputed from the updated
//     set of child states:
//   - ≥1 child still "done"  → parent becomes "partial".
//   - No child "done"         → parent becomes "pending".
//     This recomputation also fires when a child is set from "pending"/"partial"
//     to "done" but not all children are "done" (result: parent "partial").
//
//  4. Manual partial: "partial" may be set explicitly on any item (parent or
//     child) without triggering a cascade in either direction. When a child is
//     set to "partial", rule 3 fires to recompute the parent; when the parent
//     is set to "partial", no cascade to children occurs.
//
//  5. Leaf (no children): if the parent has no children, the group is a leaf.
//     The parent keeps exactly the state it is set to; no propagation occurs.
//
// # Manual-override vs auto precedence
//
// The engine is intentionally stateless: it records no "manual override" flag.
// Given the current states and a single change event it derives new states
// deterministically, so:
//
//   - Prior auto-done (rule 1) or cascade-done (rule 2) does NOT prevent rule 3
//     from recomputing the parent when a child is later unchecked. The parent
//     becomes "partial" or "pending" based on the current child aggregate.
//
//   - A parent set manually to "partial" (rule 4) will be overridden by rule 1
//     if all children subsequently transition to "done".
//
// Concrete cases:
//
//	Parent auto-done (rule 1) → child unchecked → rule 3 fires → parent partial/pending.
//	Parent cascade-done (rule 2) → child unchecked → rule 3 fires → parent partial/pending.
//	Parent manually partial (rule 4) → another child set done → if now all done, parent becomes done (rule 1).
//	Parent set pending manually → children unchanged (no cascade for pending/partial).
package rollup

// State constants for occurrence states. They match occurrences.State* so
// callers can use either package's constants.
const (
	StatePending = "pending"
	StatePartial = "partial"
	StateDone    = "done"
)

// NodeState identifies one occurrence (parent or child) by its id together
// with its current state.
type NodeState struct {
	ID    string
	State string
}

// Apply computes the complete set of state changes produced when the item
// identified by changedID transitions to newState, within the parent/children
// occurrence group.
//
// parent is the parent occurrence's current id and state.
// children holds every child occurrence and its current state (may be empty for
// a leaf parent — see rule 5).
// changedID is the id of the item being changed; it must be either parent.ID or
// a child's ID.
// newState is the desired new state for changedID (one of StatePending,
// StatePartial, StateDone).
//
// The returned slice always includes a NodeState for changedID itself.
// It additionally includes a NodeState for the parent when the parent's state
// changes as a side-effect of a child transition. The caller should persist all
// returned NodeStates.
func Apply(parent NodeState, children []NodeState, changedID, newState string) []NodeState {
	// Rule 5: leaf (no children) — apply state directly, no propagation.
	if len(children) == 0 {
		return []NodeState{{ID: changedID, State: newState}}
	}

	// The parent itself is the changed item.
	if changedID == parent.ID {
		result := []NodeState{{ID: parent.ID, State: newState}}
		if newState == StateDone {
			// Rule 2: cascade "done" to every child.
			for _, c := range children {
				result = append(result, NodeState{ID: c.ID, State: StateDone})
			}
		}
		// "partial" and "pending" do not cascade (rules 4 and natural behaviour).
		return result
	}

	// A child is the changed item — build the updated state map and recompute
	// the parent (rules 1, 3, and the side-effect of rule 4 on a child).
	updatedStates := make(map[string]string, len(children))
	for _, c := range children {
		updatedStates[c.ID] = c.State
	}
	updatedStates[changedID] = newState

	doneCount := 0
	for _, s := range updatedStates {
		if s == StateDone {
			doneCount++
		}
	}

	var newParentState string
	switch {
	case doneCount == len(children): // rule 1: all done → parent done
		newParentState = StateDone
	case doneCount > 0: // rule 3 / partial: some done → parent partial
		newParentState = StatePartial
	default: // rule 3 / pending: none done → parent pending
		newParentState = StatePending
	}

	return []NodeState{
		{ID: changedID, State: newState},
		{ID: parent.ID, State: newParentState},
	}
}
