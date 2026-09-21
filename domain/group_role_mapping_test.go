package domain

import "testing"

func TestGroupRoleMapping(t *testing.T) {
	// 無条件ロールと条件付きロールを別々に保持できる。
	group := NewGoogleGroup(email(t, "developers@example.com"), nil)
	editor := iamRole(t, "roles/editor")
	viewer := iamRole(t, "roles/viewer")
	condition := iamCondition(t, "request.time < timestamp('2030-01-01T00:00:00Z')")
	binding := NewGroupRoleBinding(viewer, condition)

	mapping := NewGroupRoleMapping(group, []IAMRole{editor}, []GroupRoleBinding{binding})

	if got, want := mapping.Group().Email(), group.Email(); got != want {
		t.Errorf("Group().Email() = %v, want %v", got, want)
	}
	if got := mapping.UnconditionalRoles(); len(got) != 1 || got[0] != editor {
		t.Errorf("UnconditionalRoles() = %v, want [%v]", got, editor)
	}
	if got := mapping.ConditionalBindings(); len(got) != 1 || got[0] != binding {
		t.Errorf("ConditionalBindings() = %v, want [%v]", got, binding)
	}
}

func TestGroupRoleMapping_slicesAreCopied(t *testing.T) {
	// コンストラクタに渡した slice と accessor が返した slice のどちらを変更しても、
	// GroupRoleMapping が保持するロールと binding は変わらない。
	group := NewGoogleGroup(email(t, "developers@example.com"), nil)
	editor := iamRole(t, "roles/editor")
	viewer := iamRole(t, "roles/viewer")
	binding := NewGroupRoleBinding(viewer, iamCondition(t, "request.time < timestamp('2030-01-01T00:00:00Z')"))

	roles := []IAMRole{editor}
	bindings := []GroupRoleBinding{binding}
	mapping := NewGroupRoleMapping(group, roles, bindings)

	roles[0] = viewer
	bindings[0] = NewGroupRoleBinding(editor, iamCondition(t, "request.time < timestamp('2031-01-01T00:00:00Z')"))
	if got := mapping.UnconditionalRoles()[0]; got != editor {
		t.Errorf("UnconditionalRoles()[0] = %v, want %v", got, editor)
	}
	if got := mapping.ConditionalBindings()[0]; got != binding {
		t.Errorf("ConditionalBindings()[0] = %v, want %v", got, binding)
	}

	returnedRoles := mapping.UnconditionalRoles()
	returnedBindings := mapping.ConditionalBindings()
	returnedRoles[0] = viewer
	returnedBindings[0] = NewGroupRoleBinding(editor, iamCondition(t, "request.time < timestamp('2031-01-01T00:00:00Z')"))
	if got := mapping.UnconditionalRoles()[0]; got != editor {
		t.Errorf("UnconditionalRoles() followed a change to its returned slice: got %v, want %v", got, editor)
	}
	if got := mapping.ConditionalBindings()[0]; got != binding {
		t.Errorf("ConditionalBindings() followed a change to its returned slice: got %v, want %v", got, binding)
	}
}

func TestNewIAMCondition(t *testing.T) {
	// 有効な CEL 式とメタデータを条件として保持できる。
	condition, err := NewIAMCondition("request.time < timestamp('2030-01-01T00:00:00Z')", "temporary", "Expires in 2030")
	if err != nil {
		t.Fatalf("NewIAMCondition() error = %v", err)
	}
	if got, want := condition.Expression(), "request.time < timestamp('2030-01-01T00:00:00Z')"; got != want {
		t.Errorf("Expression() = %q, want %q", got, want)
	}
	if got, want := condition.Title(), "temporary"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got, want := condition.Description(), "Expires in 2030"; got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
}

func TestNewIAMCondition_rejectsEmptyExpression(t *testing.T) {
	// 空白だけの CEL 式は拒否する。
	if _, err := NewIAMCondition("  ", "", ""); err == nil {
		t.Fatal("NewIAMCondition() = nil error, want error")
	}
}

func iamRole(t *testing.T, name string) IAMRole {
	t.Helper()
	role, err := NewIAMRole(name)
	if err != nil {
		t.Fatalf("NewIAMRole(%q) error = %v", name, err)
	}
	return role
}

func iamCondition(t *testing.T, expression string) IAMCondition {
	t.Helper()
	condition, err := NewIAMCondition(expression, "", "")
	if err != nil {
		t.Fatalf("NewIAMCondition(%q) error = %v", expression, err)
	}
	return condition
}
