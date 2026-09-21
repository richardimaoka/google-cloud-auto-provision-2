package domain

// BreakGlass holds the groups that retain emergency access to a project.
type BreakGlass struct {
	ultimateGroup  GoogleGroup
	emergencyGroup GoogleGroup
}

// NewBreakGlass returns BreakGlass with the given ultimate and emergency groups.
func NewBreakGlass(ultimateGroup, emergencyGroup GoogleGroup) BreakGlass {
	return BreakGlass{
		ultimateGroup:  cloneGoogleGroup(ultimateGroup),
		emergencyGroup: cloneGoogleGroup(emergencyGroup),
	}
}

// UltimateGroup returns a copy of the group with ultimate emergency access.
func (b BreakGlass) UltimateGroup() GoogleGroup { return cloneGoogleGroup(b.ultimateGroup) }

// EmergencyGroup returns a copy of the group with emergency access.
func (b BreakGlass) EmergencyGroup() GoogleGroup { return cloneGoogleGroup(b.emergencyGroup) }

func cloneGoogleGroup(group GoogleGroup) GoogleGroup {
	return NewGoogleGroup(group.Email(), group.Members())
}
