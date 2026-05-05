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
