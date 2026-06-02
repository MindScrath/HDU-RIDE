package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type CourseStore struct {
	root    string
	courses map[string]*CourseBundle
}

type CourseBundle struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Lectures    []ChapterMeta    `json:"lectures"`
	Assignments []AssignmentMeta `json:"assignments"`
	byLecture   map[string]lectureFile
	byAssign    map[string]assignmentFile
}

type ChapterMeta struct {
	ID       string        `json:"id" yaml:"id"`
	Title    string        `json:"title" yaml:"title"`
	Order    int           `json:"order" yaml:"order"`
	Sections []LectureMeta `json:"sections" yaml:"sections"`
}

type LectureMeta struct {
	ID    string `json:"id" yaml:"id"`
	File  string `json:"file" yaml:"file"`
	Title string `json:"title" yaml:"title"`
	Order int    `json:"order" yaml:"order"`
}

type AssignmentMeta struct {
	ID           string    `json:"id" yaml:"id"`
	Title        string    `json:"title" yaml:"title"`
	OpenAt       time.Time `json:"openAt"`
	DueAt        time.Time `json:"dueAt"`
	RStudioImage string    `json:"rstudioImage"`
	Starter      string    `json:"starter"`
	SubmitPath   string    `json:"submitPath"`
}

type courseYAML struct {
	ID          string        `yaml:"id"`
	Title       string        `yaml:"title"`
	Description string        `yaml:"description"`
	Chapters    []ChapterMeta `yaml:"chapters"`
	Assignments []struct {
		ID    string `yaml:"id"`
		Title string `yaml:"title"`
	} `yaml:"assignments"`
}

type assignmentYAML struct {
	ID           string `yaml:"id"`
	Title        string `yaml:"title"`
	OpenAt       string `yaml:"open_at"`
	DueAt        string `yaml:"due_at"`
	RStudioImage string `yaml:"rstudio_image"`
	Starter      string `yaml:"starter"`
	SubmitPath   string `yaml:"submit_path"`
}

type lectureFile struct {
	Meta LectureMeta
	Path string
}

type assignmentFile struct {
	Meta       AssignmentMeta
	Path       string
	ReadmePath string
}

func loadCourses(root string, defaultImage string) (*CourseStore, error) {
	store := &CourseStore{root: root, courses: map[string]*CourseBundle{}}
	pattern := filepath.Join(root, "courses", "*", "course.yml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	var allWarnings []string
	for _, file := range files {
		course, err := loadCourse(filepath.Dir(file), defaultImage)
		if err != nil && course == nil {
			allWarnings = append(allWarnings, fmt.Sprintf("skip course at %s: %v", filepath.Dir(file), err))
			continue
		}
		if err != nil {
			allWarnings = append(allWarnings, err.Error())
		}
		store.courses[course.ID] = course
	}
	if len(store.courses) == 0 {
		return store, fmt.Errorf("no courses loaded; errors: %s", strings.Join(allWarnings, "; "))
	}
	if len(allWarnings) > 0 {
		return store, fmt.Errorf("warnings: %s", strings.Join(allWarnings, "; "))
	}
	return store, nil
}

func LoadCourses(root string, defaultImage string) (*CourseStore, error) {
	return loadCourses(root, defaultImage)
}

func loadCourse(dir string, defaultImage string) (*CourseBundle, error) {
	data, err := os.ReadFile(filepath.Join(dir, "course.yml"))
	if err != nil {
		return nil, err
	}
	var manifest courseYAML
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	course := &CourseBundle{
		ID:          manifest.ID,
		Title:       manifest.Title,
		Description: manifest.Description,
		byLecture:   map[string]lectureFile{},
		byAssign:    map[string]assignmentFile{},
	}

	for _, chapter := range manifest.Chapters {
		for i, section := range chapter.Sections {
			if section.ID == "" {
				section.ID = strings.TrimSuffix(filepath.Base(section.File), filepath.Ext(section.File))
				chapter.Sections[i].ID = section.ID
			}
			lecturePath := filepath.Clean(filepath.Join(dir, section.File))
			if !strings.HasPrefix(lecturePath, filepath.Clean(dir)) {
				return nil, fmt.Errorf("lecture path escapes course directory: %s", section.File)
			}
			course.byLecture[section.ID] = lectureFile{Meta: section, Path: lecturePath}
		}
		sort.Slice(chapter.Sections, func(i, j int) bool { return chapter.Sections[i].Order < chapter.Sections[j].Order })
		course.Lectures = append(course.Lectures, chapter)
	}
	sort.Slice(course.Lectures, func(i, j int) bool { return course.Lectures[i].Order < course.Lectures[j].Order })

	var warnings []string
	for _, listed := range manifest.Assignments {
		assignDir := filepath.Join(dir, "assignments", listed.ID)
		meta, err := loadAssignment(assignDir, listed.Title, defaultImage)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skip assignment %s: %v", listed.ID, err))
			continue
		}
		course.Assignments = append(course.Assignments, meta)
		course.byAssign[meta.ID] = assignmentFile{
			Meta:       meta,
			Path:       assignDir,
			ReadmePath: filepath.Join(assignDir, "README.md"),
		}
	}
	sort.Slice(course.Assignments, func(i, j int) bool { return course.Assignments[i].OpenAt.Before(course.Assignments[j].OpenAt) })
	if len(warnings) > 0 {
		return course, fmt.Errorf("course %s loaded with warnings: %s", manifest.ID, strings.Join(warnings, "; "))
	}
	return course, nil
}

func loadAssignment(dir, listedTitle, defaultImage string) (AssignmentMeta, error) {
	data, err := os.ReadFile(filepath.Join(dir, "assignment.yml"))
	if err != nil {
		return AssignmentMeta{}, err
	}
	var raw assignmentYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return AssignmentMeta{}, err
	}
	openAt, err := parseCourseTime(raw.OpenAt)
	if err != nil {
		return AssignmentMeta{}, err
	}
	dueAt, err := parseCourseTime(raw.DueAt)
	if err != nil {
		return AssignmentMeta{}, err
	}
	if raw.Title == "" {
		raw.Title = listedTitle
	}
	if raw.RStudioImage == "" {
		raw.RStudioImage = defaultImage
	}
	return AssignmentMeta{
		ID:           raw.ID,
		Title:        raw.Title,
		OpenAt:       openAt,
		DueAt:        dueAt,
		RStudioImage: raw.RStudioImage,
		Starter:      raw.Starter,
		SubmitPath:   raw.SubmitPath,
	}, nil
}

func parseCourseTime(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(value), time.Local)
}

func (s *CourseStore) Course(id string) (*CourseBundle, bool) {
	course, ok := s.courses[id]
	return course, ok
}

func (s *CourseStore) DefaultCourse() (*CourseBundle, bool) {
	ids := make([]string, 0, len(s.courses))
	for id := range s.courses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil, false
	}
	return s.courses[ids[0]], true
}

func (s *CourseStore) Reload(defaultImage string) error {
	next, err := loadCourses(s.root, defaultImage)
	if err != nil && next == nil {
		return err
	}
	s.courses = next.courses
	if err != nil {
		return err
	}
	return nil
}

func (s *CourseStore) Courses() []*CourseBundle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*CourseBundle, 0, len(s.courses))
	for _, c := range s.courses {
		out = append(out, c)
	}
	return out
}

func (c *CourseBundle) RenderLecture(id string) (string, error) {
	item, ok := c.byLecture[id]
	if !ok {
		return "", os.ErrNotExist
	}
	return renderMarkdownFile(item.Path)
}

func (c *CourseBundle) RenderAssignment(id string) (string, AssignmentMeta, error) {
	item, ok := c.byAssign[id]
	if !ok {
		return "", AssignmentMeta{}, os.ErrNotExist
	}
	html, err := renderMarkdownFile(item.ReadmePath)
	return html, item.Meta, err
}

func renderMarkdownFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(stripFrontMatter(data)), nil
}

func stripFrontMatter(data []byte) []byte {
	text := string(data)
	if !strings.HasPrefix(text, "---") {
		return data
	}
	// Normalise line endings for delimiter detection
	normalised := strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(normalised, "---\n") {
		return data
	}
	end := strings.Index(normalised[4:], "\n---\n")
	if end < 0 {
		return data
	}
	return []byte(text[4+end+5:])
}
