package app

import "testing"

func TestRoleCapabilities(t *testing.T) {
	if !canCreateClass(User{Role: RoleTeacher}) {
		t.Fatal("teacher should create class")
	}
	if canCreateClass(User{Role: RoleAssistant}) {
		t.Fatal("assistant should not create class")
	}
	if !isAdmin(User{Role: RoleRoot}) || !isAdmin(User{Role: RoleAdmin}) {
		t.Fatal("root/admin should be admin")
	}
	if isAdmin(User{Role: RoleTeacher}) {
		t.Fatal("teacher should not be admin")
	}
}

func TestAdminUserBoundaries(t *testing.T) {
	root := User{Role: RoleRoot}
	admin := User{Role: RoleAdmin}

	if !canManageTargetUser(root, User{Role: RoleRoot}) {
		t.Fatal("root should manage root users")
	}
	if canManageTargetUser(admin, User{Role: RoleRoot}) {
		t.Fatal("admin must not manage root users")
	}
	if !canManageTargetUser(admin, User{Role: RoleTeacher}) {
		t.Fatal("admin should manage non-root users")
	}
	if !canAssignGlobalRole(root, RoleRoot) {
		t.Fatal("root should assign root role")
	}
	if canAssignGlobalRole(admin, RoleRoot) {
		t.Fatal("admin must not assign root role")
	}
	if canAssignGlobalRole(admin, Role("owner")) {
		t.Fatal("invalid role should be rejected")
	}
}
