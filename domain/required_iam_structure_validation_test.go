package domain

import (
	"slices"
	"testing"
)

func TestRequiredIAMStructure_Validate(t *testing.T) {
	owner := iamRole(t, "roles/owner")
	projectIAMAdmin := iamRole(t, "roles/resourcemanager.projectIamAdmin")
	developerRoles := []IAMRole{
		iamRole(t, "roles/viewer"),
		iamRole(t, "roles/logging.viewer"),
		iamRole(t, "roles/monitoring.viewer"),
	}
	viewer := developerRoles[0]
	required, ultimate, emergency, developers := requiredIAMStructureForValidation(t, developerRoles)

	tests := []struct {
		name     string
		actual   WholeIAMStructure
		wantErrs int
	}{
		{
			name: "各グループに必要なロールがすべて付与されていれば有効",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(ultimate, []IAMRole{owner}, nil),
				NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin}, nil),
				NewGroupRoleMapping(developers, developerRoles, nil),
			}, nil),
		},
		{
			name: "ultimateブレークグラスグループのマッピングがなければエラー",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin}, nil),
				NewGroupRoleMapping(developers, developerRoles, nil),
			}, nil),
			wantErrs: 1,
		},
		{
			name: "ultimateブレークグラスグループのロールが必要なロールと完全一致しなければエラー",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(ultimate, []IAMRole{owner, viewer}, nil),
				NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin}, nil),
				NewGroupRoleMapping(developers, developerRoles, nil),
			}, nil),
			wantErrs: 1,
		},
		{
			name: "emergencyブレークグラスグループのマッピングがなければエラー",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(ultimate, []IAMRole{owner}, nil),
				NewGroupRoleMapping(developers, developerRoles, nil),
			}, nil),
			wantErrs: 1,
		},
		{
			name: "emergencyブレークグラスグループのロールが必要なロールと完全一致しなければエラー",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(ultimate, []IAMRole{owner}, nil),
				NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin, viewer}, nil),
				NewGroupRoleMapping(developers, developerRoles, nil),
			}, nil),
			wantErrs: 1,
		},
		{
			name: "developerグループのマッピングがなければエラー",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(ultimate, []IAMRole{owner}, nil),
				NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin}, nil),
			}, nil),
			wantErrs: 1,
		},
		{
			name: "developerグループに必要なロールが一つでもなければエラー",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(ultimate, []IAMRole{owner}, nil),
				NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin}, nil),
				NewGroupRoleMapping(developers, developerRoles[:len(developerRoles)-1], nil),
			}, nil),
			wantErrs: 1,
		},
		{
			name: "developerグループに余分なロールがあればエラー",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(ultimate, []IAMRole{owner}, nil),
				NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin}, nil),
				NewGroupRoleMapping(developers, append(slices.Clone(developerRoles), owner), nil),
			}, nil),
			wantErrs: 1,
		},
		{
			name: "条件付きロールは無条件ロールの代わりにならずエラー",
			actual: NewWholeIAMStructure(nil, []GroupRoleMapping{
				NewGroupRoleMapping(ultimate, nil, []GroupRoleBinding{NewGroupRoleBinding(owner, iamCondition(t, "request.time < timestamp('2031-01-01T00:00:00Z')"))}),
				NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin}, nil),
				NewGroupRoleMapping(developers, developerRoles, nil),
			}, nil),
			wantErrs: 1,
		},
		{
			name: "必要なグループ以外にロールやprincipalが追加されていても有効",
			actual: NewWholeIAMStructure(
				[]UserRoleMapping{NewUserRoleMapping(email(t, "user@example.com"), []IAMRole{viewer}, nil)},
				[]GroupRoleMapping{
					NewGroupRoleMapping(ultimate, []IAMRole{owner}, nil),
					NewGroupRoleMapping(emergency, []IAMRole{projectIAMAdmin}, nil),
					NewGroupRoleMapping(developers, developerRoles, nil),
				},
				nil,
			),
		},
		{
			name:     "複数の不足があるときはすべてのエラーを返す",
			actual:   NewWholeIAMStructure(nil, nil, nil),
			wantErrs: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := required.Validate(tt.actual)
			if tt.wantErrs == 0 {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want %d error(s)", tt.wantErrs)
			}
			joined, ok := err.(interface{ Unwrap() []error })
			if !ok {
				t.Fatalf("Validate() = %T, want an error made by errors.Join", err)
			}
			if got := len(joined.Unwrap()); got != tt.wantErrs {
				t.Errorf("Validate() reported %d error(s), want %d: %v", got, tt.wantErrs, err)
			}
		})
	}
}

func requiredIAMStructureForValidation(t *testing.T, developerRoles []IAMRole) (RequiredIAMStructure, GoogleGroup, GoogleGroup, GoogleGroup) {
	t.Helper()
	ultimate := NewGoogleGroup(email(t, "ultimate@example.com"), nil)
	emergency := NewGoogleGroup(email(t, "emergency@example.com"), nil)
	breakGlass, err := NewBreakGlass(
		breakGlassRequirement(t, ultimate.Email().String(), emergency.Email().String()),
		ultimate,
		emergency,
	)
	if err != nil {
		t.Fatalf("NewBreakGlass() error = %v", err)
	}
	developers := NewGoogleGroup(email(t, "developers@example.com"), nil)
	required := NewRequiredIAMStructure(
		breakGlass,
		developers,
		developerRoles,
		email(t, "observer@example-project.iam.gserviceaccount.com"),
	)
	return required, ultimate, emergency, developers
}
