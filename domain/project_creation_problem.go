package domain

import "fmt"

// Problems and the recommendation each one carries:
//
//	DeveloperInUltimateBreakGlass       -> RemoveDeveloperGroupMember
//	DeveloperInEmergencyBreakGlass      -> RemoveDeveloperGroupMember
//	DeveloperGroupTooFewMembers         -> AddDeveloperGroupMembers
//	DeveloperGroupDomainMismatch        -> UseDeveloperGroupInOrganizationDomain
//	DeveloperGroupMemberDomainMismatch  -> RemoveDeveloperGroupMember

// ProjectCreationProblemCode identifies a known problem that blocks project
// creation.
type ProjectCreationProblemCode string

const (
	// ProblemDeveloperInUltimateBreakGlass: a developer is in the ultimate break-glass group.
	ProblemDeveloperInUltimateBreakGlass ProjectCreationProblemCode = "developer_group_member_in_ultimate_break_glass_group"
	// ProblemDeveloperInEmergencyBreakGlass: a developer is in the emergency break-glass group.
	ProblemDeveloperInEmergencyBreakGlass ProjectCreationProblemCode = "developer_group_member_in_emergency_break_glass_group"
	// ProblemDeveloperGroupTooFewMembers: the developer group has too few members.
	ProblemDeveloperGroupTooFewMembers ProjectCreationProblemCode = "developer_group_too_few_members"
	// ProblemDeveloperGroupDomainMismatch: the developer group is not in the organization domain.
	ProblemDeveloperGroupDomainMismatch ProjectCreationProblemCode = "developer_group_domain_mismatch"
	// ProblemDeveloperGroupMemberDomainMismatch: a developer group member is not in the organization domain.
	ProblemDeveloperGroupMemberDomainMismatch ProjectCreationProblemCode = "developer_group_member_domain_mismatch"
)

// RecommendationCode identifies the recommended action for a known problem.
type RecommendationCode string

const (
	// RecommendationRemoveDeveloperGroupMember: remove the member from the group.
	RecommendationRemoveDeveloperGroupMember RecommendationCode = "remove_developer_group_member"
	// RecommendationAddDeveloperGroupMembers: add members to the group.
	RecommendationAddDeveloperGroupMembers RecommendationCode = "add_developer_group_members"
	// RecommendationUseDeveloperGroupInOrganizationDomain: use a group in the organization domain.
	RecommendationUseDeveloperGroupInOrganizationDomain RecommendationCode = "use_developer_group_in_organization_domain"
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

// DeveloperGroupTooFewMembers is a developer group with fewer than
// minDeveloperGroupMembers members.
type DeveloperGroupTooFewMembers struct {
	developerGroup Email
	memberCount    int
}

// NewDeveloperGroupTooFewMembers returns the problem for developerGroup, which
// has memberCount members.
func NewDeveloperGroupTooFewMembers(developerGroup Email, memberCount int) DeveloperGroupTooFewMembers {
	return DeveloperGroupTooFewMembers{developerGroup: developerGroup, memberCount: memberCount}
}

func (DeveloperGroupTooFewMembers) Code() ProjectCreationProblemCode {
	return ProblemDeveloperGroupTooFewMembers
}

func (p DeveloperGroupTooFewMembers) Description() string {
	return fmt.Sprintf("developer group %s has %d member(s), want at least %d", p.developerGroup, p.memberCount, minDeveloperGroupMembers)
}

// Recommendation returns the action that resolves the problem.
func (p DeveloperGroupTooFewMembers) Recommendation() AddDeveloperGroupMembers {
	return NewAddDeveloperGroupMembers(p.developerGroup)
}

// DeveloperGroupDomainMismatch is a developer group outside the organization
// domain.
type DeveloperGroupDomainMismatch struct {
	developerGroup     Email
	organizationDomain string
}

// NewDeveloperGroupDomainMismatch returns the problem for developerGroup, which
// is not in organizationDomain.
func NewDeveloperGroupDomainMismatch(developerGroup Email, organizationDomain string) DeveloperGroupDomainMismatch {
	return DeveloperGroupDomainMismatch{developerGroup: developerGroup, organizationDomain: organizationDomain}
}

func (DeveloperGroupDomainMismatch) Code() ProjectCreationProblemCode {
	return ProblemDeveloperGroupDomainMismatch
}

func (p DeveloperGroupDomainMismatch) Description() string {
	return fmt.Sprintf("developer group %s is not in the organization domain %s", p.developerGroup, p.organizationDomain)
}

// Recommendation returns the action that resolves the problem.
func (p DeveloperGroupDomainMismatch) Recommendation() UseDeveloperGroupInOrganizationDomain {
	return NewUseDeveloperGroupInOrganizationDomain(p.organizationDomain)
}

// DeveloperGroupMemberDomainMismatch is a developer group member outside the
// organization domain.
type DeveloperGroupMemberDomainMismatch struct {
	developerGroup     Email
	member             Email
	organizationDomain string
}

// NewDeveloperGroupMemberDomainMismatch returns the problem for member of
// developerGroup, who is not in organizationDomain.
func NewDeveloperGroupMemberDomainMismatch(developerGroup, member Email, organizationDomain string) DeveloperGroupMemberDomainMismatch {
	return DeveloperGroupMemberDomainMismatch{developerGroup: developerGroup, member: member, organizationDomain: organizationDomain}
}

func (DeveloperGroupMemberDomainMismatch) Code() ProjectCreationProblemCode {
	return ProblemDeveloperGroupMemberDomainMismatch
}

func (p DeveloperGroupMemberDomainMismatch) Description() string {
	return fmt.Sprintf("member %s of developer group %s is not in the organization domain %s", p.member, p.developerGroup, p.organizationDomain)
}

// Recommendation returns the action that resolves the problem.
func (p DeveloperGroupMemberDomainMismatch) Recommendation() RemoveDeveloperGroupMember {
	return NewRemoveDeveloperGroupMember(p.member, p.developerGroup)
}

// AddDeveloperGroupMembers recommends adding members to developerGroup.
type AddDeveloperGroupMembers struct {
	developerGroup Email
}

// NewAddDeveloperGroupMembers returns the recommendation to add members to
// developerGroup.
func NewAddDeveloperGroupMembers(developerGroup Email) AddDeveloperGroupMembers {
	return AddDeveloperGroupMembers{developerGroup: developerGroup}
}

func (AddDeveloperGroupMembers) Code() RecommendationCode {
	return RecommendationAddDeveloperGroupMembers
}

func (r AddDeveloperGroupMembers) Description() string {
	return fmt.Sprintf("add members to developer group %s so it has at least %d", r.developerGroup, minDeveloperGroupMembers)
}

// UseDeveloperGroupInOrganizationDomain recommends using a developer group in
// organizationDomain.
type UseDeveloperGroupInOrganizationDomain struct {
	organizationDomain string
}

// NewUseDeveloperGroupInOrganizationDomain returns the recommendation to use a
// developer group in organizationDomain.
func NewUseDeveloperGroupInOrganizationDomain(organizationDomain string) UseDeveloperGroupInOrganizationDomain {
	return UseDeveloperGroupInOrganizationDomain{organizationDomain: organizationDomain}
}

func (UseDeveloperGroupInOrganizationDomain) Code() RecommendationCode {
	return RecommendationUseDeveloperGroupInOrganizationDomain
}

func (r UseDeveloperGroupInOrganizationDomain) Description() string {
	return fmt.Sprintf("use a developer group in the organization domain %s", r.organizationDomain)
}
