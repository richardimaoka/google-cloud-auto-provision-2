package domain

import (
	"fmt"
	"strings"
)

// Problems and the recommendation each one carries:
//
//	DeveloperInUltimateBreakGlass       -> RemoveDeveloperGroupMember
//	DeveloperInEmergencyBreakGlass      -> RemoveDeveloperGroupMember
//	DeveloperGroupTooFewMembers         -> AddDeveloperGroupMembers
//	DeveloperGroupDomainMismatch        -> UseDeveloperGroupInOrganizationDomain
//	DeveloperGroupMemberDomainMismatch  -> RemoveDeveloperGroupMember
//	RequesterDomainMismatch             -> UseRequesterInOrganizationDomain
//	ProjectIDCollision                  -> UseDifferentDisplayName

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
	// ProblemRequesterDomainMismatch: the requester is not in the organization domain.
	ProblemRequesterDomainMismatch ProjectCreationProblemCode = "requester_domain_mismatch"
	// ProblemProjectIDCollision: no project ID generated from the display name was free.
	ProblemProjectIDCollision ProjectCreationProblemCode = "project_id_collision"
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
	// RecommendationUseRequesterInOrganizationDomain: request as an account in the organization domain.
	RecommendationUseRequesterInOrganizationDomain RecommendationCode = "use_requester_in_organization_domain"
	// RecommendationUseDifferentDisplayName: request again with a different display name.
	RecommendationUseDifferentDisplayName RecommendationCode = "use_different_display_name"
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

// minDeveloperGroupMembers is the fewest members a developer group may have.
const minDeveloperGroupMembers = 2

// FindProjectCreationProblems returns every known problem that blocks creating
// the project for request, or nil if there is none. It does no I/O: the caller
// passes the developer group's members (in request), the organization and the
// break-glass groups' members as fetched from Google Cloud.
func FindProjectCreationProblems(request ProjectCreationRequest, organization Organization, breakGlass BreakGlass) []ProjectCreationProblem {
	var problems []ProjectCreationProblem
	developerGroup := request.DeveloperGroup

	if !sameDomain(request.Requester, organization) {
		problems = append(problems, NewRequesterDomainMismatch(request.Requester, organization.Domain()))
	}
	if !sameDomain(developerGroup.Email(), organization) {
		problems = append(problems, NewDeveloperGroupDomainMismatch(developerGroup.Email(), organization.Domain()))
	}

	members := developerGroup.Members()
	if len(members) < minDeveloperGroupMembers {
		problems = append(problems, NewDeveloperGroupTooFewMembers(developerGroup.Email(), len(members)))
	}
	for _, member := range members {
		if !sameDomain(member.Email(), organization) {
			problems = append(problems, NewDeveloperGroupMemberDomainMismatch(developerGroup.Email(), member.Email(), organization.Domain()))
		}
		if groupHasMember(breakGlass.UltimateGroup(), member.Email()) {
			problems = append(problems, NewDeveloperInUltimateBreakGlass(member.Email(), developerGroup.Email(), breakGlass.UltimateGroup().Email()))
		}
		if groupHasMember(breakGlass.EmergencyGroup(), member.Email()) {
			problems = append(problems, NewDeveloperInEmergencyBreakGlass(member.Email(), developerGroup.Email(), breakGlass.EmergencyGroup().Email()))
		}
	}
	return problems
}

// sameDomain reports whether e is in the organization's domain, ignoring case.
func sameDomain(e Email, organization Organization) bool {
	return strings.EqualFold(e.Domain(), organization.Domain())
}

// groupHasMember reports whether email is a member of group, ignoring case.
func groupHasMember(group GoogleGroup, email Email) bool {
	for _, m := range group.Members() {
		if strings.EqualFold(m.Email().String(), email.String()) {
			return true
		}
	}
	return false
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

// RequesterDomainMismatch is a requester outside the organization domain.
type RequesterDomainMismatch struct {
	requester          Email
	organizationDomain string
}

// NewRequesterDomainMismatch returns the problem for requester, who is not in
// organizationDomain.
func NewRequesterDomainMismatch(requester Email, organizationDomain string) RequesterDomainMismatch {
	return RequesterDomainMismatch{requester: requester, organizationDomain: organizationDomain}
}

func (RequesterDomainMismatch) Code() ProjectCreationProblemCode {
	return ProblemRequesterDomainMismatch
}

func (p RequesterDomainMismatch) Description() string {
	return fmt.Sprintf("requester %s is not in the organization domain %s", p.requester, p.organizationDomain)
}

// Recommendation returns the action that resolves the problem.
func (p RequesterDomainMismatch) Recommendation() UseRequesterInOrganizationDomain {
	return NewUseRequesterInOrganizationDomain(p.organizationDomain)
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

// UseRequesterInOrganizationDomain recommends requesting as an account in
// organizationDomain.
type UseRequesterInOrganizationDomain struct {
	organizationDomain string
}

// NewUseRequesterInOrganizationDomain returns the recommendation to request as
// an account in organizationDomain.
func NewUseRequesterInOrganizationDomain(organizationDomain string) UseRequesterInOrganizationDomain {
	return UseRequesterInOrganizationDomain{organizationDomain: organizationDomain}
}

func (UseRequesterInOrganizationDomain) Code() RecommendationCode {
	return RecommendationUseRequesterInOrganizationDomain
}

func (r UseRequesterInOrganizationDomain) Description() string {
	return fmt.Sprintf("request as an account in the organization domain %s", r.organizationDomain)
}

// Problems below are discovered only once a request is sent to Google Cloud,
// unlike the problems above, which FindProjectCreationProblems finds from the
// request alone.

// ProjectIDCollision is a project creation whose display name produced no free
// project ID: every ID Google Cloud generated from it was already taken.
type ProjectIDCollision struct {
	displayName string
	attempts    int
}

// NewProjectIDCollision returns the problem for displayName, for which
// attempts project IDs were tried and all were already taken.
func NewProjectIDCollision(displayName string, attempts int) ProjectIDCollision {
	return ProjectIDCollision{displayName: displayName, attempts: attempts}
}

func (ProjectIDCollision) Code() ProjectCreationProblemCode {
	return ProblemProjectIDCollision
}

func (p ProjectIDCollision) Description() string {
	return fmt.Sprintf("no project ID generated from display name %q was free, tried %d time(s)", p.displayName, p.attempts)
}

// Recommendation returns the action that resolves the problem.
func (p ProjectIDCollision) Recommendation() UseDifferentDisplayName {
	return NewUseDifferentDisplayName(p.displayName)
}

// UseDifferentDisplayName recommends requesting again with a display name
// other than displayName.
type UseDifferentDisplayName struct {
	displayName string
}

// NewUseDifferentDisplayName returns the recommendation to request again with
// a display name other than displayName.
func NewUseDifferentDisplayName(displayName string) UseDifferentDisplayName {
	return UseDifferentDisplayName{displayName: displayName}
}

func (UseDifferentDisplayName) Code() RecommendationCode {
	return RecommendationUseDifferentDisplayName
}

func (r UseDifferentDisplayName) Description() string {
	return fmt.Sprintf("request again with a display name other than %q", r.displayName)
}
