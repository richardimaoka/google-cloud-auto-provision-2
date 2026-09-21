package domain

import "testing"

func TestNewBreakGlass(t *testing.T) {
	// 要件に一致する 2 グループから BreakGlass を作成できる。
	ultimateMember := NewGoogleGroupMember(email(t, "ultimate-member@example.com"), "MEMBER")
	emergencyMember := NewGoogleGroupMember(email(t, "emergency-member@example.com"), "MEMBER")
	ultimateGroup := NewGoogleGroup(email(t, "ultimate@example.com"), []GoogleGroupMember{ultimateMember})
	emergencyGroup := NewGoogleGroup(email(t, "emergency@example.com"), []GoogleGroupMember{emergencyMember})
	requirement := breakGlassRequirement(t, ultimateGroup.Email().String(), emergencyGroup.Email().String())

	breakGlass, err := NewBreakGlass(requirement, ultimateGroup, emergencyGroup)
	if err != nil {
		t.Fatalf("NewBreakGlass() error = %v", err)
	}
	if got := breakGlass.UltimateGroup(); got.Email() != ultimateGroup.Email() || got.Members()[0] != ultimateMember {
		t.Errorf("UltimateGroup() = %#v, want %#v", got, ultimateGroup)
	}
	if got := breakGlass.EmergencyGroup(); got.Email() != emergencyGroup.Email() || got.Members()[0] != emergencyMember {
		t.Errorf("EmergencyGroup() = %#v, want %#v", got, emergencyGroup)
	}
}

func TestBreakGlass_GroupsAreCopied(t *testing.T) {
	// 入力と accessor のグループを変更しても BreakGlass の状態は変わらない。
	member := NewGoogleGroupMember(email(t, "member@example.com"), "MEMBER")
	other := NewGoogleGroupMember(email(t, "other@example.com"), "MEMBER")
	ultimateGroup := NewGoogleGroup(email(t, "ultimate@example.com"), []GoogleGroupMember{member})
	emergencyGroup := NewGoogleGroup(email(t, "emergency@example.com"), nil)
	breakGlass, err := NewBreakGlass(breakGlassRequirement(t, ultimateGroup.Email().String(), emergencyGroup.Email().String()), ultimateGroup, emergencyGroup)
	if err != nil {
		t.Fatalf("NewBreakGlass() error = %v", err)
	}

	breakGlass.ultimateGroup.members[0] = other
	if got := ultimateGroup.Members()[0]; got != member {
		t.Errorf("input group changed to %v, want %v", got, member)
	}

	returnedGroup := breakGlass.UltimateGroup()
	returnedGroup.members[0] = member
	if got := breakGlass.UltimateGroup().Members()[0]; got != other {
		t.Errorf("BreakGlass followed a change to a returned group: got %v, want %v", got, other)
	}
}

func TestNewBreakGlassRequirement(t *testing.T) {
	// 2 つの環境変数を読み込み、それぞれを対応する非公開フィールドに保存する。
	t.Setenv(BreakGlassUltimateGroupEmailEnv, "ultimate@example.com")
	t.Setenv(BreakGlassEmergencyGroupEmailEnv, "emergency@example.com")
	requirement, err := NewBreakGlassRequirement()
	if err != nil {
		t.Fatalf("NewBreakGlassRequirement() error = %v", err)
	}
	if got, want := requirement.ultimateGroupEmail.String(), "ultimate@example.com"; got != want {
		t.Errorf("ultimateGroupEmail = %q, want %q", got, want)
	}
	if got, want := requirement.emergencyGroupEmail.String(), "emergency@example.com"; got != want {
		t.Errorf("emergencyGroupEmail = %q, want %q", got, want)
	}
}

func TestNewBreakGlassRequirement_rejectsMissingEnvironmentVariable(t *testing.T) {
	// 必須の環境変数が欠けている場合は要件を作成できない。
	t.Setenv(BreakGlassUltimateGroupEmailEnv, "ultimate@example.com")
	t.Setenv(BreakGlassEmergencyGroupEmailEnv, "")
	if _, err := NewBreakGlassRequirement(); err == nil {
		t.Fatal("NewBreakGlassRequirement() = nil error, want error")
	}
}

func TestNewBreakGlass_rejectsWrongGroups(t *testing.T) {
	// 要件と異なるメールアドレスのグループは拒否する。
	requirement := breakGlassRequirement(t, "ultimate@example.com", "emergency@example.com")
	ultimate := NewGoogleGroup(email(t, "wrong-ultimate@example.com"), nil)
	emergency := NewGoogleGroup(email(t, "emergency@example.com"), nil)
	if _, err := NewBreakGlass(requirement, ultimate, emergency); err == nil {
		t.Fatal("NewBreakGlass() = nil error, want error")
	}
}

func TestNewBreakGlass_rejectsZeroRequirement(t *testing.T) {
	// 設定されていないゼロ値の要件は拒否する。
	ultimate := NewGoogleGroup(email(t, "ultimate@example.com"), nil)
	emergency := NewGoogleGroup(email(t, "emergency@example.com"), nil)
	if _, err := NewBreakGlass(BreakGlassRequirement{}, ultimate, emergency); err == nil {
		t.Fatal("NewBreakGlass() = nil error, want error")
	}
}

func breakGlassRequirement(t *testing.T, ultimate, emergency string) BreakGlassRequirement {
	t.Helper()
	t.Setenv(BreakGlassUltimateGroupEmailEnv, ultimate)
	t.Setenv(BreakGlassEmergencyGroupEmailEnv, emergency)
	requirement, err := NewBreakGlassRequirement()
	if err != nil {
		t.Fatalf("NewBreakGlassRequirement() error = %v", err)
	}
	return requirement
}
