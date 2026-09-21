package domain

import "fmt"

// ProjectCreationProblemCode identifies a known problem that blocks project
// creation.
type ProjectCreationProblemCode string

const (
	// ProblemDeveloperInUltimateBreakGlass: a developer is in the ultimate break-glass group.
	ProblemDeveloperInUltimateBreakGlass ProjectCreationProblemCode = "developer_group_member_in_ultimate_break_glass_group"
	// ProblemDeveloperInEmergencyBreakGlass: a developer is in the emergency break-glass group.
	ProblemDeveloperInEmergencyBreakGlass ProjectCreationProblemCode = "developer_group_member_in_emergency_break_glass_group"
)

// RecommendationCode identifies the recommended action for a known problem.
type RecommendationCode string

const (
	// RecommendationRemoveDeveloperGroupMember: remove the member from the group.
	RecommendationRemoveDeveloperGroupMember RecommendationCode = "remove_developer_group_member"
)

// ProjectCreationProblem is a known problem that blocks project creation. Each
// problem code has its own implementing type holding that problem's facts.
type ProjectCreationProblem interface {
	Code() ProjectCreationProblemCode
	// Description returns a human-readable explanation of the problem.
	Description() string
}

// Recommendation is the recommended action for a ProjectCreationProblem. Each
// recommendation code has its own implementing type holding the action's targets.
type Recommendation interface {
	Code() RecommendationCode
	// Description returns a human-readable explanation of the action.
	Description() string
}

// DeveloperInUltimateBreakGlass is a developer group member who is also in the
// ultimate break-glass group.
type DeveloperInUltimateBreakGlass struct {
	member          Email
	developerGroup  Email
	breakGlassGroup Email
}

// NewDeveloperInUltimateBreakGlass returns the problem for member, who belongs
// to developerGroup and breakGlassGroup.
func NewDeveloperInUltimateBreakGlass(member, developerGroup, breakGlassGroup Email) DeveloperInUltimateBreakGlass {
	return DeveloperInUltimateBreakGlass{member: member, developerGroup: developerGroup, breakGlassGroup: breakGlassGroup}
}

func (DeveloperInUltimateBreakGlass) Code() ProjectCreationProblemCode {
	return ProblemDeveloperInUltimateBreakGlass
}

func (p DeveloperInUltimateBreakGlass) Description() string {
	return fmt.Sprintf("%s in developer group %s is also in the ultimate break-glass group %s", p.member, p.developerGroup, p.breakGlassGroup)
}

// Recommendation returns the action that resolves the problem.
func (p DeveloperInUltimateBreakGlass) Recommendation() RemoveDeveloperGroupMember {
	return NewRemoveDeveloperGroupMember(p.member, p.developerGroup)
}

// DeveloperInEmergencyBreakGlass is a developer group member who is also in the
// emergency break-glass group.
type DeveloperInEmergencyBreakGlass struct {
	member          Email
	developerGroup  Email
	breakGlassGroup Email
}

// NewDeveloperInEmergencyBreakGlass returns the problem for member, who belongs
// to developerGroup and breakGlassGroup.
func NewDeveloperInEmergencyBreakGlass(member, developerGroup, breakGlassGroup Email) DeveloperInEmergencyBreakGlass {
	return DeveloperInEmergencyBreakGlass{member: member, developerGroup: developerGroup, breakGlassGroup: breakGlassGroup}
}

func (DeveloperInEmergencyBreakGlass) Code() ProjectCreationProblemCode {
	return ProblemDeveloperInEmergencyBreakGlass
}

func (p DeveloperInEmergencyBreakGlass) Description() string {
	return fmt.Sprintf("%s in developer group %s is also in the emergency break-glass group %s", p.member, p.developerGroup, p.breakGlassGroup)
}

// Recommendation returns the action that resolves the problem.
func (p DeveloperInEmergencyBreakGlass) Recommendation() RemoveDeveloperGroupMember {
	return NewRemoveDeveloperGroupMember(p.member, p.developerGroup)
}

// RemoveDeveloperGroupMember recommends removing member from developerGroup.
type RemoveDeveloperGroupMember struct {
	member         Email
	developerGroup Email
}

// NewRemoveDeveloperGroupMember returns the recommendation to remove member
// from developerGroup.
func NewRemoveDeveloperGroupMember(member, developerGroup Email) RemoveDeveloperGroupMember {
	return RemoveDeveloperGroupMember{member: member, developerGroup: developerGroup}
}

func (RemoveDeveloperGroupMember) Code() RecommendationCode {
	return RecommendationRemoveDeveloperGroupMember
}

func (r RemoveDeveloperGroupMember) Description() string {
	return fmt.Sprintf("remove %s from developer group %s", r.member, r.developerGroup)
}

// Member returns the member to remove.
func (r RemoveDeveloperGroupMember) Member() Email { return r.member }

// DeveloperGroup returns the group to remove the member from.
func (r RemoveDeveloperGroupMember) DeveloperGroup() Email { return r.developerGroup }
