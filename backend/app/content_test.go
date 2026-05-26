package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCoursesAndRenderMarkdown(t *testing.T) {
	root := t.TempDir()
	courseDir := filepath.Join(root, "courses", "intro-r")
	writeTestFile(t, filepath.Join(courseDir, "course.yml"), `
id: intro-r
title: Intro R
description: Course
chapters:
  - id: ch1
    title: Chapter 1
    order: 1
    sections:
      - id: welcome
        file: chapters/01-welcome.md
        title: Welcome
        order: 1
assignments:
  - id: hw01
    title: Homework 1
`)
	writeTestFile(t, filepath.Join(courseDir, "chapters", "01-welcome.md"), "---\ntitle: Welcome\n---\n\n# Hello\n\n<script>alert(1)</script>\n")
	writeTestFile(t, filepath.Join(courseDir, "assignments", "hw01", "assignment.yml"), `
id: hw01
title: Homework 1
open_at: 2026-04-01 09:00
due_at: 2026-04-15 23:59
starter: starter
submit_path: .
`)
	writeTestFile(t, filepath.Join(courseDir, "assignments", "hw01", "README.md"), "# HW01")

	store, err := loadCourses(root, "rocker/rstudio:4.6.0")
	if err != nil {
		t.Fatal(err)
	}
	course, ok := store.Course("intro-r")
	if !ok {
		t.Fatal("course missing")
	}
	if len(course.Lectures) != 1 || len(course.Lectures[0].Sections) != 1 || len(course.Assignments) != 1 {
		t.Fatalf("unexpected course shape: %+v", course)
	}
	markdown, err := course.RenderLecture("welcome")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(markdown, "title: Welcome") {
		t.Fatalf("front matter was rendered: %s", markdown)
	}
	if !strings.Contains(markdown, "<script>") {
		t.Fatalf("raw markdown should contain unmodified script tag: %s", markdown)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0644); err != nil {
		t.Fatal(err)
	}
}
