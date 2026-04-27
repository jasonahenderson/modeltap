package harnessshell

// Pending-permission navigation and decision state. Stage C wires the
// shell-side pending-permission lifecycle alongside the
// PermissionRequestedEvent / PermissionResolvedEvent boundary.

import "fmt"

// currentPendingPermission returns a pointer to the active pending
// permission, clamping the active index to the valid range. It returns nil
// when no permissions are pending.
func (s *state) currentPendingPermission() *PendingPermission {
	if len(s.pendingPermissions) == 0 {
		return nil
	}
	if s.activePermissionIndex < 0 {
		s.activePermissionIndex = 0
	}
	if s.activePermissionIndex >= len(s.pendingPermissions) {
		s.activePermissionIndex = len(s.pendingPermissions) - 1
	}
	return &s.pendingPermissions[s.activePermissionIndex]
}

// movePermissionAction advances the selected permission action by delta and
// clamps it to the [0, 2] range. Returns true iff a permission was active.
func (s *state) movePermissionAction(delta int) bool {
	p := s.currentPendingPermission()
	if p == nil {
		return false
	}
	p.SelectedAction += delta
	if p.SelectedAction < 0 {
		p.SelectedAction = 0
	}
	if p.SelectedAction > 2 {
		p.SelectedAction = 2
	}
	s.status = "Permission action selected"
	return true
}

// movePendingPermission switches the active pending permission by delta.
// Returns true when navigation occurred (only when there is more than one
// pending permission).
func (s *state) movePendingPermission(delta int) bool {
	if len(s.pendingPermissions) < 2 {
		return false
	}
	s.activePermissionIndex += delta
	if s.activePermissionIndex < 0 {
		s.activePermissionIndex = 0
	}
	if s.activePermissionIndex >= len(s.pendingPermissions) {
		s.activePermissionIndex = len(s.pendingPermissions) - 1
	}
	s.status = fmt.Sprintf("Pending permission %d of %d", s.activePermissionIndex+1, len(s.pendingPermissions))
	return true
}

// removeActivePendingPermission removes the active pending permission and
// returns it. The returned bool is false when there were no pending
// permissions.
func (s *state) removeActivePendingPermission() (PendingPermission, bool) {
	if len(s.pendingPermissions) == 0 {
		return PendingPermission{}, false
	}
	if s.activePermissionIndex < 0 {
		s.activePermissionIndex = 0
	}
	if s.activePermissionIndex >= len(s.pendingPermissions) {
		s.activePermissionIndex = len(s.pendingPermissions) - 1
	}
	p := s.pendingPermissions[s.activePermissionIndex]
	s.pendingPermissions = append(s.pendingPermissions[:s.activePermissionIndex], s.pendingPermissions[s.activePermissionIndex+1:]...)
	if len(s.pendingPermissions) == 0 {
		s.activePermissionIndex = 0
	} else if s.activePermissionIndex >= len(s.pendingPermissions) {
		s.activePermissionIndex = len(s.pendingPermissions) - 1
	}
	return p, true
}

// removePendingPermissionByID removes a pending permission whose request ID
// matches the given id, regardless of its position. Returns true if a
// permission was removed.
func (s *state) removePendingPermissionByID(id string) bool {
	for i := range s.pendingPermissions {
		if s.pendingPermissions[i].Request.ID == id {
			s.pendingPermissions = append(s.pendingPermissions[:i], s.pendingPermissions[i+1:]...)
			if len(s.pendingPermissions) == 0 {
				s.activePermissionIndex = 0
			} else if s.activePermissionIndex >= len(s.pendingPermissions) {
				s.activePermissionIndex = len(s.pendingPermissions) - 1
			}
			return true
		}
	}
	return false
}

// permissionDecisionFromAction maps the composer's selected-action index to
// a typed [PermissionDecision].
func permissionDecisionFromAction(idx int) PermissionDecision {
	switch idx {
	case 0:
		return DecisionApproveOnce
	case 1:
		return DecisionApproveSession
	default:
		return DecisionDeny
	}
}

// resolveActivePermission emits a [ResolvePermissionAction] for the
// currently-active pending permission with the given decision. The host
// owns the terminal outcome — the shell does not pre-decide locally; the
// transcript event row flips to granted/denied only on
// [PermissionResolvedEvent] intake.
func (s *state) resolveActivePermission(decision PermissionDecision) {
	p := s.currentPendingPermission()
	if p == nil {
		return
	}
	s.pendingActions = append(s.pendingActions, ResolvePermissionAction{
		RequestID: p.Request.ID,
		Decision:  decision,
	})
	s.status = "Resolving permission"
	s.statusKind = StatusPermissionPending
}
