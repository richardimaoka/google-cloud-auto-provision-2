package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
)

// ProjectCreationProblem is the persistence-neutral representation of a known,
// actionable project-creation problem. Code and Recommendation are suitable for
// storing in separate queryable columns. DetailsJSON holds the problem-specific
// facts as a JSON object.
type ProjectCreationProblem struct {
	code           domain.ProjectCreationProblemCode
	recommendation domain.RecommendationCode
	detailsJSON    json.RawMessage
}

// NewProjectCreationProblem returns a ProjectCreationProblem with the given
// code, recommendation, and problem-specific details. detailsJSON must be a
// valid JSON object.
func NewProjectCreationProblem(code domain.ProjectCreationProblemCode, recommendation domain.RecommendationCode, detailsJSON json.RawMessage) (ProjectCreationProblem, error) {
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

// NewDeveloperGroupMemberInUltimateBreakGlassGroupProblem returns the problem
// and the recommendation for a developer group member who also belongs to the
// ultimate break-glass group.
func NewDeveloperGroupMemberInUltimateBreakGlassGroupProblem(member, breakGlassGroup domain.Email) (ProjectCreationProblem, error) {
	return newDeveloperGroupMemberInBreakGlassGroupProblem(domain.ProblemDeveloperGroupMemberInUltimateBreakGlassGroup, member, breakGlassGroup)
}

// NewDeveloperGroupMemberInEmergencyBreakGlassGroupProblem returns the problem
// and the recommendation for a developer group member who also belongs to the
// emergency break-glass group.
func NewDeveloperGroupMemberInEmergencyBreakGlassGroupProblem(member, breakGlassGroup domain.Email) (ProjectCreationProblem, error) {
	return newDeveloperGroupMemberInBreakGlassGroupProblem(domain.ProblemDeveloperGroupMemberInEmergencyBreakGlassGroup, member, breakGlassGroup)
}

func newDeveloperGroupMemberInBreakGlassGroupProblem(code domain.ProjectCreationProblemCode, member, breakGlassGroup domain.Email) (ProjectCreationProblem, error) {
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
		code,
		domain.RecommendationRemoveDeveloperGroupMember,
		detailsJSON,
	)
}

func (p ProjectCreationProblem) Code() domain.ProjectCreationProblemCode { return p.code }

func (p ProjectCreationProblem) Recommendation() domain.RecommendationCode { return p.recommendation }

// DetailsJSON returns a copy of the problem-specific JSON object.
func (p ProjectCreationProblem) DetailsJSON() json.RawMessage {
	return append(json.RawMessage(nil), p.detailsJSON...)
}
