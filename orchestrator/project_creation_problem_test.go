package orchestrator

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yusni/google-cloud-auto-provision-2/domain"
)

func TestNewProjectCreationProblem(t *testing.T) {
	details := json.RawMessage(`{"member":"alice@example.com","break_glass_group":"break-glass@example.com"}`)

	tests := []struct {
		name           string
		code           domain.ProjectCreationProblemCode
		recommendation domain.RecommendationCode
		details        json.RawMessage
		wantErr        bool
	}{
		{name: "code、recommendation、JSONの詳細を保持する", code: domain.ProblemDeveloperInUltimateBreakGlass, recommendation: domain.RecommendationRemoveDeveloperGroupMember, details: details},
		{name: "codeが空だとエラー", recommendation: domain.RecommendationRemoveDeveloperGroupMember, details: details, wantErr: true},
		{name: "recommendationが空だとエラー", code: domain.ProblemDeveloperInUltimateBreakGlass, details: details, wantErr: true},
		{name: "不正なJSONだとエラー", code: domain.ProblemDeveloperInUltimateBreakGlass, recommendation: domain.RecommendationRemoveDeveloperGroupMember, details: json.RawMessage(`{`), wantErr: true},
		{name: "JSONオブジェクト以外はエラー", code: domain.ProblemDeveloperInUltimateBreakGlass, recommendation: domain.RecommendationRemoveDeveloperGroupMember, details: json.RawMessage(`[]`), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem, err := NewProjectCreationProblem(tt.code, tt.recommendation, tt.details)
			if tt.wantErr {
				if err == nil {
					t.Fatal("NewProjectCreationProblem() = nil error, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewProjectCreationProblem(): %v", err)
			}
			if problem.Code() != tt.code || problem.Recommendation() != tt.recommendation {
				t.Errorf("Code()/Recommendation() = %q/%q, want %q/%q", problem.Code(), problem.Recommendation(), tt.code, tt.recommendation)
			}
			if !reflect.DeepEqual(problem.DetailsJSON(), tt.details) {
				t.Errorf("DetailsJSON() = %s, want %s", problem.DetailsJSON(), tt.details)
			}
		})
	}
}

func TestProjectCreationProblem_DetailsJSONReturnsCopy(t *testing.T) {
	input := json.RawMessage(`{"member":"alice@example.com"}`)
	problem, err := NewProjectCreationProblem("example", "example_recommendation", input)
	if err != nil {
		t.Fatal(err)
	}

	input[2] = 'X'
	if got := problem.DetailsJSON(); !reflect.DeepEqual(got, json.RawMessage(`{"member":"alice@example.com"}`)) {
		t.Errorf("problem followed a change to the input: %s", got)
	}

	output := problem.DetailsJSON()
	output[2] = 'X'
	if got := problem.DetailsJSON(); !reflect.DeepEqual(got, json.RawMessage(`{"member":"alice@example.com"}`)) {
		t.Errorf("problem followed a change to the returned JSON: %s", got)
	}
}

func TestNewDeveloperGroupMemberInBreakGlassGroupProblems(t *testing.T) {
	tests := []struct {
		name     string
		new      func(member, breakGlassGroup domain.Email) (ProjectCreationProblem, error)
		wantCode domain.ProjectCreationProblemCode
	}{
		{name: "ultimate", new: NewDeveloperGroupMemberInUltimateBreakGlassGroupProblem, wantCode: domain.ProblemDeveloperInUltimateBreakGlass},
		{name: "emergency", new: NewDeveloperGroupMemberInEmergencyBreakGlassGroupProblem, wantCode: domain.ProblemDeveloperInEmergencyBreakGlass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			member, err := domain.NewEmail("alice@example.com")
			if err != nil {
				t.Fatal(err)
			}
			breakGlassGroup, err := domain.NewEmail("break-glass@example.com")
			if err != nil {
				t.Fatal(err)
			}
			problem, err := tt.new(member, breakGlassGroup)
			if err != nil {
				t.Fatal(err)
			}
			if problem.Code() != tt.wantCode {
				t.Errorf("Code() = %q, want %q", problem.Code(), tt.wantCode)
			}
			if problem.Recommendation() != domain.RecommendationRemoveDeveloperGroupMember {
				t.Errorf("Recommendation() = %q, want %q", problem.Recommendation(), domain.RecommendationRemoveDeveloperGroupMember)
			}
			wantDetails := json.RawMessage(`{"member":"alice@example.com","break_glass_group":"break-glass@example.com"}`)
			if !reflect.DeepEqual(problem.DetailsJSON(), wantDetails) {
				t.Errorf("DetailsJSON() = %s, want %s", problem.DetailsJSON(), wantDetails)
			}
		})
	}
}
