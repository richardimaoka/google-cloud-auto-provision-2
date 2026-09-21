package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
)

// ProjectCreationProblemCode identifies a known problem that prevents project
// creation from proceeding.
type ProjectCreationProblemCode string

const (
	// ProblemDeveloperGroupMemberInBreakGlassGroup reports that a developer
	// group member is also a member of a break-glass group.
	ProblemDeveloperGroupMemberInBreakGlassGroup ProjectCreationProblemCode = "developer_group_member_in_break_glass_group"
)

// RecommendationCode identifies the recommended action for a known problem.
// It is stored with the problem as the recommendation that was presented when
// the problem occurred.
type RecommendationCode string

const (
	// RecommendationRemoveDeveloperGroupMember advises removing the overlapping
	// member from the developer group.
	RecommendationRemoveDeveloperGroupMember RecommendationCode = "remove_developer_group_member"
)

// ProjectCreationProblem is the persistence-neutral representation of a known,
// actionable project-creation problem. Code and Recommendation are suitable for
// storing in separate queryable columns. DetailsJSON holds the problem-specific
// facts as a JSON object.
type ProjectCreationProblem struct {
	code           ProjectCreationProblemCode
	recommendation RecommendationCode
	detailsJSON    json.RawMessage
}

// NewProjectCreationProblem returns a ProjectCreationProblem with the given
// code, recommendation, and problem-specific details. detailsJSON must be a
// valid JSON object.
func NewProjectCreationProblem(code ProjectCreationProblemCode, recommendation RecommendationCode, detailsJSON json.RawMessage) (ProjectCreationProblem, error) {
	if code == "" {
		return ProjectCreationProblem{}, errors.New("project creation problem code is empty")
	}
	if recommendation == "" {
		return ProjectCreationProblem{}, errors.New("project creation problem recommendation is empty")
	}
	var details map[string]json.RawMessage
	if err := json.Unmarshal(detailsJSON, &details); err != nil {
		return ProjectCreationProblem{}, fmt.Errorf("invalid project creation problem details JSON: %w", err)
	}
	if details == nil {
		return ProjectCreationProblem{}, errors.New("project creation problem details JSON must be an object")
	}
	return ProjectCreationProblem{
		code:           code,
		recommendation: recommendation,
		detailsJSON:    append(json.RawMessage(nil), detailsJSON...),
	}, nil
}

// NewDeveloperGroupMemberInBreakGlassGroupProblem returns the problem and the
// recommendation for a developer group member who also belongs to a
// break-glass group.
func NewDeveloperGroupMemberInBreakGlassGroupProblem(member, breakGlassGroup domain.Email) (ProjectCreationProblem, error) {
	detailsJSON, err := json.Marshal(struct {
		Member          string `json:"member"`
		BreakGlassGroup string `json:"break_glass_group"`
	}{
		Member:          member.String(),
		BreakGlassGroup: breakGlassGroup.String(),
	})
	if err != nil {
		return ProjectCreationProblem{}, fmt.Errorf("marshal developer group member break-glass problem details: %w", err)
	}
	return NewProjectCreationProblem(
		ProblemDeveloperGroupMemberInBreakGlassGroup,
		RecommendationRemoveDeveloperGroupMember,
		detailsJSON,
	)
}

func (p ProjectCreationProblem) Code() ProjectCreationProblemCode { return p.code }

func (p ProjectCreationProblem) Recommendation() RecommendationCode { return p.recommendation }

// DetailsJSON returns a copy of the problem-specific JSON object.
func (p ProjectCreationProblem) DetailsJSON() json.RawMessage {
	return append(json.RawMessage(nil), p.detailsJSON...)
}
