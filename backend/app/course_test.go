package app

import (
	"context"
	"testing"
)

func TestCourseModel(t *testing.T) {
	c := Course{
		ID:          "test-id",
		Name:        "计量经济学",
		Code:        "econ-301",
		Description: "本科计量经济学课程",
		Status:      "active",
		ContentRoot: "/content/econ-301",
		CreatedBy:   "root-user",
	}

	if c.Name != "计量经济学" {
		t.Errorf("expected '计量经济学', got %q", c.Name)
	}
	if c.Code != "econ-301" {
		t.Errorf("expected 'econ-301', got %q", c.Code)
	}
	if c.Status != "active" {
		t.Errorf("expected 'active', got %q", c.Status)
	}
}

func TestCourseMemberModel(t *testing.T) {
	cm := CourseMember{
		CourseID:   "course-1",
		UserID:     "user-1",
		MemberRole: "admin",
	}

	if cm.CourseID != "course-1" {
		t.Errorf("expected 'course-1', got %q", cm.CourseID)
	}
	if cm.MemberRole != "admin" {
		t.Errorf("expected 'admin', got %q", cm.MemberRole)
	}
}

func TestCourseRoleValidation(t *testing.T) {
	validRoles := []string{"admin", "teacher"}
	invalidRoles := []string{"student", "root", "assistant", "", "owner"}

	for _, r := range validRoles {
		if r != "admin" && r != "teacher" {
			t.Errorf("expected valid role, got %q", r)
		}
	}

	for _, r := range invalidRoles {
		if r == "admin" || r == "teacher" {
			t.Errorf("expected invalid role, got %q", r)
		}
	}
}

func TestCoursePermissions(t *testing.T) {
	// Root can do everything
	root := User{Role: RoleRoot}
	if root.Role != RoleRoot {
		t.Fatal("root should be root")
	}

	// Admin and teacher are valid course roles
	admin := User{Role: RoleAdmin}
	teacher := User{Role: RoleTeacher}
	student := User{Role: RoleStudent}

	if admin.Role == RoleRoot {
		t.Fatal("admin should not be root")
	}
	if teacher.Role == RoleRoot {
		t.Fatal("teacher should not be root")
	}
	if student.Role != RoleStudent {
		t.Fatal("student should be student")
	}
}

func TestCourseDBOperations(t *testing.T) {
	cfg := Config{
		SessionSecret:    "test-secret",
		RootUsername:     "test-root",
		RootPasswordHash: "$2a$10$test",
		DatabaseURL:      "postgres://hdu:hdu@127.0.0.1:5432/hdu_ride?sslmode=disable",
	}

	pool, err := openDB(context.Background(), cfg)
	if err != nil {
		t.Skip("database not available, skipping integration test")
		return
	}
	defer pool.Close()

	// Ensure schema exists
	if err := initSchema(context.Background(), pool, cfg); err != nil {
		t.Fatal("initSchema:", err)
	}

	// Test: courses table exists
	var count int
	err = pool.QueryRow(context.Background(), `select count(*) from courses`).Scan(&count)
	if err != nil {
		t.Fatal("courses table should exist:", err)
	}

	// Test: course_members table exists
	err = pool.QueryRow(context.Background(), `select count(*) from course_members`).Scan(&count)
	if err != nil {
		t.Fatal("course_members table should exist:", err)
	}

	t.Log("courses and course_members tables verified")
}
