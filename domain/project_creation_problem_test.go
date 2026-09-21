package domain

import (
	"slices"
	"testing"
)

func TestFindProjectCreationProblems(t *testing.T) {
	org, err := NewOrganization("123456789012", "example.com")
	if err != nil {
		t.Fatal(err)
	}

	group := func(t *testing.T, groupEmail string, members ...string) GoogleGroup {
		t.Helper()
		var ms []GoogleGroupMember
		for _, m := range members {
			ms = append(ms, NewGoogleGroupMember(email(t, m), "MEMBER"))
		}
		return NewGoogleGroup(email(t, groupEmail), ms)
	}

	tests := []struct {
		name      string
		requester string
		// The developer group's email; developer-group@example.com if empty.
		groupEmail string
		developer  []string // members of the developer group
		ultimate   []string // members of the ultimate break-glass group
		emergency  []string // members of the emergency break-glass group
		// The codes of the problems, in the order they are reported.
		want []ProjectCreationProblemCode
	}{
		{
			name:      "問題がなければnil",
			requester: "requester@example.com",
			developer: []string{"member-1@example.com", "member-2@example.com"},
		},
		{
			name:      "requesterのドメインが違う",
			requester: "requester@other.com",
			developer: []string{"member-1@example.com", "member-2@example.com"},
			want:      []ProjectCreationProblemCode{ProblemRequesterDomainMismatch},
		},
		{
			name:      "ドメインの大文字小文字が違っても同じドメインとして扱う",
			requester: "requester@EXAMPLE.com",
			developer: []string{"member-1@EXAMPLE.COM", "member-2@Example.com"},
		},
		{
			name:       "開発者グループのドメインが違う",
			requester:  "requester@example.com",
			groupEmail: "developer-group@other.com",
			developer:  []string{"member-1@example.com", "member-2@example.com"},
			want:       []ProjectCreationProblemCode{ProblemDeveloperGroupDomainMismatch},
		},
		{
			name:      "メンバーが0人",
			requester: "requester@example.com",
			want:      []ProjectCreationProblemCode{ProblemDeveloperGroupTooFewMembers},
		},
		{
			name:      "メンバーが2人未満",
			requester: "requester@example.com",
			developer: []string{"member-1@example.com"},
			want:      []ProjectCreationProblemCode{ProblemDeveloperGroupTooFewMembers},
		},
		{
			name:      "メンバーのドメインが違う",
			requester: "requester@example.com",
			developer: []string{"member-1@example.com", "outside@other.com"},
			want:      []ProjectCreationProblemCode{ProblemDeveloperGroupMemberDomainMismatch},
		},
		{
			name:      "ultimate break-glassグループにも入っているメンバー",
			requester: "requester@example.com",
			developer: []string{"member-1@example.com", "member-2@example.com"},
			ultimate:  []string{"member-1@example.com"},
			want:      []ProjectCreationProblemCode{ProblemDeveloperInUltimateBreakGlass},
		},
		{
			name:      "emergency break-glassグループにも入っているメンバー（大文字小文字は区別しない）",
			requester: "requester@example.com",
			developer: []string{"member-1@example.com", "member-2@example.com"},
			emergency: []string{"Member-2@Example.com"},
			want:      []ProjectCreationProblemCode{ProblemDeveloperInEmergencyBreakGlass},
		},
		{
			name:      "重なったときはすべて報告する",
			requester: "requester@other.com",
			developer: []string{"member-1@example.com"},
			ultimate:  []string{"member-1@example.com"},
			emergency: []string{"member-1@example.com"},
			want: []ProjectCreationProblemCode{
				ProblemRequesterDomainMismatch,
				ProblemDeveloperGroupTooFewMembers,
				ProblemDeveloperInUltimateBreakGlass,
				ProblemDeveloperInEmergencyBreakGlass,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupEmail := tt.groupEmail
			if groupEmail == "" {
				groupEmail = "developer-group@example.com"
			}
			developerGroup := group(t, groupEmail, tt.developer...)
			request, err := NewProjectCreationRequest("Example Project", email(t, tt.requester), developerGroup)
			if err != nil {
				t.Fatal(err)
			}
			requirement := BreakGlassRequirement{
				ultimateGroupEmail:  email(t, "break-glass-ultimate@example.com"),
				emergencyGroupEmail: email(t, "break-glass-emergency@example.com"),
			}
			breakGlass, err := NewBreakGlass(requirement,
				group(t, "break-glass-ultimate@example.com", tt.ultimate...),
				group(t, "break-glass-emergency@example.com", tt.emergency...),
			)
			if err != nil {
				t.Fatal(err)
			}

			var got []ProjectCreationProblemCode
			for _, p := range FindProjectCreationProblems(request, org, breakGlass) {
				got = append(got, p.Code())
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("FindProjectCreationProblems() codes = %v, want %v", got, tt.want)
			}
		})
	}
}
