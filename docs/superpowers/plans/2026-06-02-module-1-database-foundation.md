# Module 1: Database Foundation — courses + course_members

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `courses` and `course_members` tables to PostgreSQL, auto-migrate existing data, update Go models and routes, add frontend types.

**Architecture:** New tables hold course metadata and course-scoped member roles. Existing `classes.course_id` stays as text FK to `courses.id`. Class creators are auto-enrolled as course teachers.

**Tech Stack:** Go 1.26 + PostgreSQL (pgx) + Next.js 16 + React 19 + TypeScript

---

### Task 1: Add Go models for Course and CourseMember

**Files:**
- Modify: `backend/app/models.go`

- [ ] **Step 1: Add Course and CourseMember structs**

Add after the existing `Class` struct (around line 32):

```go
type Course struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	ContentRoot string    `json:"contentRoot"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type CourseMember struct {
	CourseID   string    `json:"courseId"`
	UserID     string    `json:"userId"`
	MemberRole string    `json:"memberRole"` // 'admin' | 'teacher'
	JoinedAt   time.Time `json:"joinedAt"`
	InvitedBy  string    `json:"invitedBy"`
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/app/models.go
git commit -m "feat: add Course and CourseMember models"
```

---

### Task 2: Add DDL for courses and course_members tables

**Files:**
- Modify: `backend/app/db.go:26-105`

- [ ] **Step 1: Add CREATE TABLE statements to initSchema**

Insert BEFORE the existing `create table if not exists classes` block (around line 45):

```go
create table if not exists courses (
  id text primary key,
  name text not null,
  code text not null unique,
  description text not null default '',
  status text not null check (status in ('active','archived')) default 'active',
  content_root text not null default '',
  created_by text not null references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists course_members (
  course_id text not null references courses(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  member_role text not null check (member_role in ('admin','teacher')),
  joined_at timestamptz not null default now(),
  invited_by text references users(id),
  primary key (course_id, user_id)
);
```

- [ ] **Step 2: Add data migration logic after DDL**

Add after the root user insert (after line 114), before the closing of `initSchema`:

```go
// Migrate: create courses from existing classes
_, _ = db.Exec(ctx, `
insert into courses (id, name, code, description, status, created_by)
select distinct on (c.course_id)
  gen_random_uuid(),
  c.course_id,
  c.course_id,
  '',
  'active',
  c.created_by
from classes c
where not exists (select 1 from courses co where co.code = c.course_id)
`)

// Migrate: enroll class creators as course teachers
_, _ = db.Exec(ctx, `
insert into course_members (course_id, user_id, member_role)
select distinct on (co.id, c.created_by)
  co.id,
  c.created_by,
  'teacher'
from classes c
join courses co on co.code = c.course_id
where not exists (
  select 1 from course_members cm
  where cm.course_id = co.id and cm.user_id = c.created_by
)
`)
```

- [ ] **Step 3: Commit**

```bash
git add backend/app/db.go
git commit -m "feat: add courses and course_members DDL with data migration"
```

---

### Task 3: Add CourseStore methods for course access

**Files:**
- Modify: `backend/app/content.go`

- [ ] **Step 1: Add Courses() accessor method to CourseStore**

Add after the `Reload` method (around line 229):

```go
func (s *CourseStore) Courses() []*CourseBundle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*CourseBundle, 0, len(s.courses))
	for _, c := range s.courses {
		out = append(out, c)
	}
	return out
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/app/content.go
git commit -m "feat: add Courses() accessor to CourseStore"
```

---

### Task 4: Add course management API endpoints

**Files:**
- Modify: `backend/app/routes.go`

- [ ] **Step 1: Add course list endpoint — GET /api/admin/courses**

Add before `registerRoutes` (or in the admin routes section):

```go
func (a *App) listCourses(c *gin.Context) {
	user := currentUser(c)
	rows, err := a.db.Query(c.Request.Context(), `
select co.id, co.name, co.code, co.description, co.status, co.content_root,
       co.created_by, co.created_at, co.updated_at,
       (select count(*) from course_members cm where cm.course_id = co.id) as member_count,
       (select count(*) from classes cl where cl.course_id = co.code) as class_count
from courses co
where exists (
  select 1 from course_members cm where cm.course_id = co.id and cm.user_id = $1
) or $2 in ('root','admin')
order by co.created_at desc
`, user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	type courseRow struct {
		ID           string    `json:"id"`
		Name         string    `json:"name"`
		Code         string    `json:"code"`
		Description  string    `json:"description"`
		Status       string    `json:"status"`
		ContentRoot  string    `json:"contentRoot"`
		CreatedBy    string    `json:"createdBy"`
		CreatedAt    time.Time `json:"createdAt"`
		UpdatedAt    time.Time `json:"updatedAt"`
		MemberCount  int       `json:"memberCount"`
		ClassCount   int       `json:"classCount"`
	}
	var courses []courseRow
	for rows.Next() {
		var r courseRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Code, &r.Description, &r.Status, &r.ContentRoot, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &r.MemberCount, &r.ClassCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "scan failed"})
			return
		}
		courses = append(courses, r)
	}
	c.JSON(http.StatusOK, gin.H{"courses": courses})
}
```

- [ ] **Step 2: Add course create endpoint — POST /api/admin/courses**

```go
func (a *App) createCourse(c *gin.Context) {
	user := currentUser(c)
	if user.Role != RoleRoot {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅 root 可创建课程"})
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required"`
		Code        string `json:"code" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid course request"})
		return
	}
	id := uuid.NewString()
	_, err := a.db.Exec(c.Request.Context(), `
insert into courses (id, name, code, description, created_by)
values ($1,$2,$3,$4,$5)
`, id, req.Name, req.Code, req.Description, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "课程创建失败，code 可能重复"})
		return
	}
	// 创建者自动成为课程 admin
	_, _ = a.db.Exec(c.Request.Context(), `
insert into course_members (course_id, user_id, member_role, invited_by)
values ($1,$2,'admin',$3)
on conflict do nothing
`, id, user.ID, user.ID)
	logEvent(c.Request.Context(), a.db, user.ID, "course.create", id)
	c.JSON(http.StatusCreated, gin.H{"id": id})
}
```

- [ ] **Step 3: Add course update endpoint — PATCH /api/admin/courses/:id**

```go
func (a *App) updateCourse(c *gin.Context) {
	user := currentUser(c)
	courseID := c.Param("courseID")
	if !a.isCourseAdmin(c.Request.Context(), user.ID, courseID) && user.Role != RoleRoot {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅课程管理员可编辑"})
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Name != nil {
		_, _ = a.db.Exec(c.Request.Context(), `update courses set name=$1, updated_at=now() where id=$2`, *req.Name, courseID)
	}
	if req.Description != nil {
		_, _ = a.db.Exec(c.Request.Context(), `update courses set description=$1, updated_at=now() where id=$2`, *req.Description, courseID)
	}
	if req.Status != nil {
		_, _ = a.db.Exec(c.Request.Context(), `update courses set status=$1, updated_at=now() where id=$2`, *req.Status, courseID)
	}
	logEvent(c.Request.Context(), a.db, user.ID, "course.update", courseID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

- [ ] **Step 4: Add course member list — GET /api/admin/courses/:id/members**

```go
func (a *App) listCourseMembers(c *gin.Context) {
	user := currentUser(c)
	courseID := c.Param("courseID")
	if !a.isCourseAdmin(c.Request.Context(), user.ID, courseID) && user.Role != RoleRoot {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	rows, err := a.db.Query(c.Request.Context(), `
select cm.user_id, u.username, u.display_name, u.role, cm.member_role, cm.joined_at, cm.invited_by
from course_members cm
join users u on u.id = cm.user_id
where cm.course_id = $1
order by cm.joined_at desc
`, courseID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()
	type memberRow struct {
		UserID      string    `json:"userId"`
		Username    string    `json:"username"`
		DisplayName string    `json:"displayName"`
		GlobalRole  string    `json:"globalRole"`
		MemberRole  string    `json:"memberRole"`
		JoinedAt    time.Time `json:"joinedAt"`
		InvitedBy   string    `json:"invitedBy"`
	}
	var members []memberRow
	for rows.Next() {
		var m memberRow
		if err := rows.Scan(&m.UserID, &m.Username, &m.DisplayName, &m.GlobalRole, &m.MemberRole, &m.JoinedAt, &m.InvitedBy); err != nil {
			continue
		}
		members = append(members, m)
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}
```

- [ ] **Step 5: Add course member add/remove — POST/DELETE /api/admin/courses/:id/members**

```go
func (a *App) addCourseMember(c *gin.Context) {
	user := currentUser(c)
	courseID := c.Param("courseID")
	if !a.isCourseAdmin(c.Request.Context(), user.ID, courseID) && user.Role != RoleRoot {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		UserID string `json:"userId" binding:"required"`
		Role   string `json:"role" binding:"required"` // 'admin' | 'teacher'
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Role != "admin" && req.Role != "teacher" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be admin or teacher"})
		return
	}
	_, err := a.db.Exec(c.Request.Context(), `
insert into course_members (course_id, user_id, member_role, invited_by)
values ($1,$2,$3,$4)
on conflict (course_id, user_id) do update set member_role=$3
`, courseID, req.UserID, req.Role, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加成员失败"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "course.add_member", courseID+":"+req.UserID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) removeCourseMember(c *gin.Context) {
	user := currentUser(c)
	courseID := c.Param("courseID")
	targetUserID := c.Param("userID")
	if !a.isCourseAdmin(c.Request.Context(), user.ID, courseID) && user.Role != RoleRoot {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if user.ID == targetUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能移除自己"})
		return
	}
	_, _ = a.db.Exec(c.Request.Context(), `
delete from course_members where course_id=$1 and user_id=$2
`, courseID, targetUserID)
	logEvent(c.Request.Context(), a.db, user.ID, "course.remove_member", courseID+":"+targetUserID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

- [ ] **Step 6: Add isCourseAdmin helper method to App**

Add to `routes.go` (or better, to `auth.go`):

In `backend/app/auth.go`, add:

```go
func (a *App) isCourseAdmin(ctx context.Context, userID, courseID string) bool {
	var exists bool
	err := a.db.QueryRow(ctx, `
select exists(select 1 from course_members where course_id=$1 and user_id=$2 and member_role='admin')
`, courseID, userID).Scan(&exists)
	return err == nil && exists
}

func (a *App) isCourseTeacher(ctx context.Context, userID, courseID string) bool {
	var exists bool
	err := a.db.QueryRow(ctx, `
select exists(select 1 from course_members where course_id=$1 and user_id=$2 and member_role in ('admin','teacher'))
`, courseID, userID).Scan(&exists)
	return err == nil && exists
}

func (a *App) courseIDForClass(ctx context.Context, classID string) (string, error) {
	// Look up course code from class, then resolve to course ID
	var courseCode string
	err := a.db.QueryRow(ctx, `select course_id from classes where id=$1`, classID).Scan(&courseCode)
	if err != nil {
		return "", err
	}
	var courseID string
	err = a.db.QueryRow(ctx, `select id from courses where code=$1`, courseCode).Scan(&courseID)
	return courseID, err
}
```

- [ ] **Step 7: Register new routes**

In the `registerRoutes` function, add under the admin routes section:

```go
api.GET("/admin/courses", app.listCourses)
api.POST("/admin/courses", app.createCourse)
api.PATCH("/admin/courses/:courseID", app.updateCourse)
api.GET("/admin/courses/:courseID/members", app.listCourseMembers)
api.POST("/admin/courses/:courseID/members", app.addCourseMember)
api.DELETE("/admin/courses/:courseID/members/:userID", app.removeCourseMember)
```

- [ ] **Step 8: Add time import (if not already)**

Check that `time` is imported in `routes.go`. If not, add to imports.

- [ ] **Step 9: Commit**

```bash
git add backend/app/routes.go backend/app/auth.go
git commit -m "feat: add course management API endpoints"
```

---

### Task 5: Add frontend types for Course and CourseMember

**Files:**
- Modify: `frontend-react/lib/types.ts`

- [ ] **Step 1: Add Course and CourseMember interfaces**

```typescript
export interface Course {
  id: string
  name: string
  code: string
  description: string
  status: 'active' | 'archived'
  contentRoot: string
  createdBy: string
  createdAt: string
  updatedAt: string
  memberCount: number
  classCount: number
}

export interface CourseMember {
  userId: string
  username: string
  displayName: string
  globalRole: Role
  memberRole: 'admin' | 'teacher'
  joinedAt: string
  invitedBy: string
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend-react/lib/types.ts
git commit -m "feat: add Course and CourseMember frontend types"
```

---

### Task 6: Write tests for course API

**Files:**
- Create: `backend/app/course_test.go`

- [ ] **Step 1: Write test file**

```go
package app

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestCourseCreate(t *testing.T) {
	cfg := testConfig(t)
	db := testDB(t, cfg)
	defer db.Close()

	ctx := context.Background()

	// Create root user
	rootID := uuid.NewString()
	_, err := db.Exec(ctx, `insert into users (id, username, display_name, password_hash, role, status) values ($1,'root-test','Root','hash','root','active')`, rootID)
	if err != nil {
		t.Fatal(err)
	}

	// Create course
	courseID := uuid.NewString()
	_, err = db.Exec(ctx, `insert into courses (id, name, code, description, created_by) values ($1,'Test Course','test-101','A test','root-test')`, courseID)
	if err != nil {
		t.Fatal(err)
	}

	// Verify course exists
	var name string
	err = db.QueryRow(ctx, `select name from courses where id=$1`, courseID).Scan(&name)
	if err != nil {
		t.Fatal("course not found:", err)
	}
	if name != "Test Course" {
		t.Errorf("expected 'Test Course', got %q", name)
	}
}

func TestCourseMemberEnrollment(t *testing.T) {
	cfg := testConfig(t)
	db := testDB(t, cfg)
	defer db.Close()

	ctx := context.Background()

	// Create users
	rootID := uuid.NewString()
	teacherID := uuid.NewString()
	db.Exec(ctx, `insert into users (id, username, display_name, password_hash, role, status) values ($1,'root2','Root','hash','root','active')`, rootID)
	db.Exec(ctx, `insert into users (id, username, display_name, password_hash, role, status) values ($1,'teacher1','Teacher','hash','teacher','active')`, teacherID)

	// Create course
	courseID := uuid.NewString()
	db.Exec(ctx, `insert into courses (id, name, code, description, created_by) values ($1,'Math','math-202','','root2')`, courseID)

	// Enroll teacher
	_, err := db.Exec(ctx, `insert into course_members (course_id, user_id, member_role) values ($1,$2,'teacher') on conflict do nothing`, courseID, teacherID)
	if err != nil {
		t.Fatal("failed to enroll teacher:", err)
	}

	// Verify enrollment
	var role string
	err = db.QueryRow(ctx, `select member_role from course_members where course_id=$1 and user_id=$2`, courseID, teacherID).Scan(&role)
	if err != nil {
		t.Fatal("enrollment not found:", err)
	}
	if role != "teacher" {
		t.Errorf("expected 'teacher', got %q", role)
	}
}
```

- [ ] **Step 2: Add test helpers**

Add to `backend/app/course_test.go` at top:

```go
func testConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		SessionSecret:    "test-secret",
		RootUsername:     "root-test",
		RootPasswordHash: "$2a$10$placeholder",
	}
}

func testDB(t *testing.T, cfg Config) *pgxpool.Pool {
	t.Helper()
	pool, err := openDB(context.Background(), cfg)
	if err != nil {
		t.Fatal("openDB:", err)
	}
	if err := initSchema(context.Background(), pool, cfg); err != nil {
		pool.Close()
		t.Fatal("initSchema:", err)
	}
	return pool
}
```

- [ ] **Step 3: Run tests**

```bash
cd backend
go test ./app/ -run "TestCourse" -v
```

Expected: 2 tests PASS

- [ ] **Step 4: Commit**

```bash
git add backend/app/course_test.go
git commit -m "test: add course and course_member tests"
```

---

### Task 7: Verify existing tests still pass

- [ ] **Step 1: Run full test suite**

```bash
cd backend
go test ./app/ -v
```

Expected: All existing tests + new course tests PASS

- [ ] **Step 2: Fix any breakage**

If existing tests break due to the new DDL / migration, fix and retest.

- [ ] **Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix: test compatibility after courses DDL"
```

---

### Task 8: Push and verify

- [ ] **Step 1: Push**

```bash
git push origin feat/react
```

- [ ] **Step 2: Rebuild backend Docker image (on cloud server)**

```bash
cd ~/hdu-ride && git pull
sudo docker build -t hdu-ride-backend:latest -f deploy/docker/backend.Dockerfile .
sudo docker save hdu-ride-backend:latest -o /tmp/backend.tar
sudo ctr -n k8s.io images import /tmp/backend.tar
kubectl rollout restart deployment/hdu-ride-backend -n hdu-ride
kubectl get pods -n hdu-ride -w
```

- [ ] **Step 3: Verify courses API**

```bash
# On cloud server, after pods are ready
kubectl exec -n hdu-ride deploy/hdu-ride-backend -- wget -qO- http://localhost:8080/api/admin/courses
```

Expected: Returns JSON with migrated courses (from existing classes).

---

### Summary of changes

| File | Action |
|------|--------|
| `backend/app/models.go` | Add Course, CourseMember structs |
| `backend/app/db.go` | Add DDL + data migration |
| `backend/app/content.go` | Add Courses() method |
| `backend/app/routes.go` | Add 6 course management endpoints |
| `backend/app/auth.go` | Add isCourseAdmin, isCourseTeacher helpers |
| `backend/app/course_test.go` | Add tests |
| `frontend-react/lib/types.ts` | Add Course, CourseMember types |
