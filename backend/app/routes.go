package app

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	cfg        Config
	db         *pgxpool.Pool
	content    *CourseStore
	objects    *ObjectStore
	workspaces *WorkspaceManager
}

func NewApp(cfg Config, db *pgxpool.Pool, content *CourseStore, objects *ObjectStore, workspaces *WorkspaceManager) *App {
	return &App{cfg: cfg, db: db, content: content, objects: objects, workspaces: workspaces}
}

func registerRoutes(router *gin.Engine, app *App) {
	router.POST("/api/login", app.login)

	api := router.Group("/api", requireSession(app.db, app.cfg))
	api.GET("/session", app.session)
	api.POST("/logout", app.logout)
	api.PATCH("/me/password", app.changePassword)
	api.GET("/lectures", app.listGlobalLectures)
	api.GET("/lectures/:lectureID", app.getGlobalLecture)
	api.GET("/classes", app.listClasses)
	api.POST("/classes", app.createClass)
	api.POST("/classes/bulk", app.bulkClasses)
	api.GET("/classes/:classID", app.getClass)
	api.DELETE("/classes/:classID", app.deleteClass)
	api.GET("/classes/:classID/teachers", app.listClassTeachers)
	api.POST("/classes/:classID/teachers", app.addClassTeacher)
	api.DELETE("/classes/:classID/teachers/:userID", app.removeClassTeacher)
	api.GET("/classes/:classID/members", app.listMembers)
	api.POST("/classes/:classID/members/import", app.importMembers)
	api.POST("/classes/:classID/members/bulk", app.bulkMembers)
	api.DELETE("/classes/:classID/members/:userID", app.removeMember)
	api.POST("/classes/:classID/members/:userID/password", app.resetMemberPassword)
	api.POST("/classes/:classID/assistants", app.assignAssistant)
	api.GET("/classes/:classID/lectures", app.listLectures)
	api.GET("/classes/:classID/lectures/:lectureID", app.getLecture)
	api.GET("/classes/:classID/assignments", app.listAssignments)
	api.GET("/classes/:classID/assignments/:assignmentID", app.getAssignment)
	api.POST("/classes/:classID/assignments/:assignmentID/submit", app.submitAssignment)
	api.GET("/classes/:classID/assignments/:assignmentID/submissions", app.listSubmissions)
	api.GET("/classes/:classID/assignments/:assignmentID/grades/export", app.exportGrades)
	api.POST("/classes/:classID/assignments/:assignmentID/workspace", app.startWorkspace)
	api.GET("/submissions/:id", app.getSubmission)
	api.GET("/submissions/:id/preview", app.previewSubmission)
	api.GET("/submissions/:id/archive", app.downloadSubmissionArchive)
	api.POST("/submissions/:id/workspace", app.startSubmissionWorkspace)
	api.POST("/submissions/:id/grade", app.gradeSubmission)
	api.POST("/grades/:id/publish", app.publishGrade)
	api.DELETE("/workspaces/:id", app.stopWorkspace)
	api.POST("/workspaces/:id/heartbeat", app.heartbeatWorkspace)
	api.GET("/admin/users", app.listUsers)
	api.POST("/admin/users", app.createUser)
	api.POST("/admin/users/bulk", app.bulkUsers)
	api.PATCH("/admin/users/:id", app.updateUser)
	api.DELETE("/admin/users/:id", app.deleteUser)
	api.POST("/admin/users/:id/password", app.resetUserPassword)
	api.POST("/admin/courses/import", app.importCourse)
	api.POST("/admin/courses/reload", app.reloadCourses)
	api.GET("/admin/courses", app.listCourses)
	api.POST("/admin/courses", app.createCourse)
	api.PATCH("/admin/courses/:courseID", app.updateCourse)
	api.GET("/admin/courses/:courseID/members", app.listCourseMembers)
	api.POST("/admin/courses/:courseID/members", app.addCourseMember)
	api.DELETE("/admin/courses/:courseID/members/:userID", app.removeCourseMember)
	api.POST("/ai/chat", app.chatAI)
	api.POST("/ai/upload", app.uploadAIFile)

	router.Any("/ide/s/:workspaceID/*path", requireSession(app.db, app.cfg), workspaceGateway(app.db, app.cfg))
}

func RegisterRoutes(router *gin.Engine, app *App) {
	registerRoutes(router, app)
}

func (a *App) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid login request"})
		return
	}

	var user User
	var passwordHash string
	err := a.db.QueryRow(c.Request.Context(), `
select id, username, display_name, password_hash, role, status, created_at
from users where username=$1 and status='active'
`, req.Username).Scan(&user.ID, &user.Username, &user.DisplayName, &passwordHash, &user.Role, &user.Status, &user.CreatedAt)
	if err != nil || !checkPassword(passwordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	token, err := createSession(c.Request.Context(), a.db, a.cfg, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session create failed"})
		return
	}
	setSessionCookie(c, a.cfg, token)
	logEvent(c.Request.Context(), a.db, user.ID, "login", user.Username)
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (a *App) session(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": currentUser(c)})
}

func (a *App) logout(c *gin.Context) {
	token, _ := c.Cookie(sessionCookie)
	deleteSession(c.Request.Context(), a.db, a.cfg, token)
	clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) changePassword(c *gin.Context) {
	user := currentUser(c)
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.NewPassword) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password request"})
		return
	}

	var passwordHash string
	err := a.db.QueryRow(c.Request.Context(), `select password_hash from users where id=$1 and status='active'`, user.ID).Scan(&passwordHash)
	if err != nil || !checkPassword(passwordHash, req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid old password"})
		return
	}
	nextHash, err := hashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password hash failed"})
		return
	}
	if _, err := a.db.Exec(c.Request.Context(), `update users set password_hash=$1 where id=$2`, nextHash, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password update failed"})
		return
	}
	token, _ := c.Cookie(sessionCookie)
	_, _ = a.db.Exec(c.Request.Context(), `delete from sessions where user_id=$1 and token_hash<>$2`, user.ID, hashToken(a.cfg, token))
	logEvent(c.Request.Context(), a.db, user.ID, "user.password.change", user.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) listClasses(c *gin.Context) {
	user := currentUser(c)
	query := `select id, course_id, name, term, note, created_by, created_at from classes order by created_at desc`
	args := []any{}
	if user.Role == RoleTeacher {
		query = `select id, course_id, name, term, note, created_by, created_at from classes where created_by=$1 order by created_at desc`
		args = append(args, user.ID)
	}
	if user.Role == RoleAssistant || user.Role == RoleStudent {
		query = `select c.id, c.course_id, c.name, c.term, c.note, c.created_by, c.created_at
from classes c join class_members m on m.class_id=c.id where m.user_id=$1 order by c.created_at desc`
		args = append(args, user.ID)
	}

	rows, err := a.db.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "classes query failed"})
		return
	}
	defer rows.Close()
	classes := []Class{}
	for rows.Next() {
		var item Class
		if err := rows.Scan(&item.ID, &item.CourseID, &item.Name, &item.Term, &item.Note, &item.CreatedBy, &item.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "classes scan failed"})
			return
		}
		classes = append(classes, item)
	}
	c.JSON(http.StatusOK, gin.H{"classes": classes})
}

func (a *App) createClass(c *gin.Context) {
	user := currentUser(c)
	if !canCreateClass(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅教师和管理员可创建班级"})
		return
	}
	var req struct {
		CourseID   string   `json:"courseId" binding:"required"`
		Name       string   `json:"name" binding:"required"`
		Term       string   `json:"term" binding:"required"`
		Note       string   `json:"note"`
		TeacherIDs []string `json:"teacherIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid class request"})
		return
	}
	// 非 root/admin 必须属于该课程的 teacher/admin
	if user.Role != RoleRoot && user.Role != RoleAdmin {
		ctx := c.Request.Context()
		courseID, err := a.courseIDFromCode(ctx, req.CourseID)
		if err != nil || !a.isCourseTeacher(ctx, user.ID, courseID) {
			c.JSON(http.StatusForbidden, gin.H{"error": "仅课程教师可在该课程下创建班级"})
			return
		}
	}
	// 即使课程尚未导入，也允许创建班级（讲义/作业列表会显示为空）
	classID := uuid.NewString()
	_, err := a.db.Exec(c.Request.Context(), `
insert into classes (id, course_id, name, term, note, created_by) values ($1,$2,$3,$4,$5,$6)
`, classID, req.CourseID, req.Name, req.Term, req.Note, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "class create failed"})
		return
	}
	// 创建者自动成为班级教师
	_, _ = a.db.Exec(c.Request.Context(), `insert into class_teachers (class_id, user_id) values ($1,$2) on conflict do nothing`, classID, user.ID)
	for _, tid := range req.TeacherIDs {
		if tid != user.ID {
			_, _ = a.db.Exec(c.Request.Context(), `insert into class_teachers (class_id, user_id) values ($1,$2) on conflict do nothing`, classID, tid)
		}
	}
	logEvent(c.Request.Context(), a.db, user.ID, "class.create", classID)
	c.JSON(http.StatusCreated, gin.H{"id": classID})
}

func (a *App) bulkClasses(c *gin.Context) {
	user := currentUser(c)
	var req struct {
		IDs    []string `json:"ids" binding:"required"`
		Action string   `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Action != "delete" || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid class bulk request"})
		return
	}
	deleted := 0
	for _, classID := range uniqueStrings(req.IDs) {
		if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		removed, err := a.deleteClassByID(c.Request.Context(), classID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "class delete failed"})
			return
		}
		if removed {
			deleted++
			logEvent(c.Request.Context(), a.db, user.ID, "class.delete", classID)
		}
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

func (a *App) getClass(c *gin.Context) {
	classID := c.Param("classID")
	if ok, err := canAccessClass(c.Request.Context(), a.db, currentUser(c), classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var item Class
	err := a.db.QueryRow(c.Request.Context(), `
select id, course_id, name, term, note, created_by, created_at from classes where id=$1
`, classID).Scan(&item.ID, &item.CourseID, &item.Name, &item.Term, &item.Note, &item.CreatedBy, &item.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "class not found"})
		return
	}
	course, _ := a.content.Course(item.CourseID)
	c.JSON(http.StatusOK, gin.H{"class": item, "course": course})
}

func (a *App) deleteClass(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	deleted, err := a.deleteClassByID(c.Request.Context(), classID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "class delete failed"})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "class not found"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "class.delete", classID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) listMembers(c *gin.Context) {
	classID := c.Param("classID")
	if ok, err := canAccessClass(c.Request.Context(), a.db, currentUser(c), classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	rows, err := a.db.Query(c.Request.Context(), `
select u.id, u.username, u.display_name, u.role, u.status, u.created_at, m.member_role, m.joined_at
from class_members m join users u on u.id=m.user_id
where m.class_id=$1
order by m.member_role, u.username
`, classID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "members query failed"})
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var user User
		var memberRole string
		var joinedAt time.Time
		if err := rows.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &user.Status, &user.CreatedAt, &memberRole, &joinedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "members scan failed"})
			return
		}
		items = append(items, gin.H{"user": user, "memberRole": memberRole, "joinedAt": joinedAt})
	}
	c.JSON(http.StatusOK, gin.H{"members": items})
}

func (a *App) importMembers(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	students, err := parseStudentImport(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member import"})
		return
	}
	for _, student := range students {
		hash, err := hashPassword(student.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "password hash failed"})
			return
		}
		userID := uuid.NewString()
		var role Role
		err = a.db.QueryRow(c.Request.Context(), `
insert into users (id, username, display_name, password_hash, role, status)
values ($1,$2,$3,$4,'student','active')
on conflict (username) do update set display_name=excluded.display_name, status='active'
where users.role in ('student','assistant')
returning id, role
`, userID, student.Username, student.DisplayName, hash).Scan(&userID, &role)
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "member import only supports student or assistant accounts"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "student upsert failed"})
			return
		}
		_, err = a.db.Exec(c.Request.Context(), `
insert into class_members (class_id, user_id, member_role) values ($1,$2,'student')
on conflict (class_id, user_id) do update set member_role='student'
`, classID, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "member bind failed"})
			return
		}
		// 助教角色仅通过 class_members.member_role 管理，不修改全局角色
	}
	logEvent(c.Request.Context(), a.db, user.ID, "class.members.import", classID)
	c.JSON(http.StatusOK, gin.H{"imported": len(students)})
}

func (a *App) removeMember(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	userID := c.Param("userID")
	if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	removed, err := a.removeClassMember(c.Request.Context(), classID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "member remove failed"})
		return
	}
	if !removed {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "class.member.remove", classID+":"+userID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) bulkMembers(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		UserIDs    []string `json:"userIds" binding:"required"`
		Action     string   `json:"action" binding:"required"`
		MemberRole string   `json:"memberRole"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member bulk request"})
		return
	}

	count := 0
	switch req.Action {
	case "remove":
		for _, userID := range uniqueStrings(req.UserIDs) {
			removed, err := a.removeClassMember(c.Request.Context(), classID, userID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "member remove failed"})
				return
			}
			if removed {
				count++
			}
		}
	case "setMemberRole":
		if req.MemberRole != "student" && req.MemberRole != "assistant" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member role"})
			return
		}
		for _, userID := range uniqueStrings(req.UserIDs) {
			updated, err := a.setClassMemberRole(c.Request.Context(), classID, userID, req.MemberRole)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "member role update failed"})
				return
			}
			if !updated {
				c.JSON(http.StatusBadRequest, gin.H{"error": "member role update not allowed"})
				return
			}
			count++
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member bulk action"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "class.members.bulk", classID+":"+req.Action)
	c.JSON(http.StatusOK, gin.H{"updated": count})
}

func (a *App) resetMemberPassword(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	userID := c.Param("userID")
	if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password request"})
		return
	}
	var target User
	err := a.db.QueryRow(c.Request.Context(), `
select u.id, u.username, u.display_name, u.role, u.status, u.created_at
from class_members m join users u on u.id=m.user_id
where m.class_id=$1 and m.user_id=$2 and m.member_role='student'
`, classID, userID).Scan(&target.ID, &target.Username, &target.DisplayName, &target.Role, &target.Status, &target.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "student member not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "member query failed"})
		return
	}
	if target.Role != RoleStudent {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := a.updateUserPassword(c.Request.Context(), userID, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password reset failed"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "class.member.password.reset", classID+":"+userID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) assignAssistant(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		UserID string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid assistant request"})
		return
	}
	updated, err := a.setClassMemberRole(c.Request.Context(), classID, req.UserID, "assistant")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "assistant assign failed"})
		return
	}
	if !updated {
		c.JSON(http.StatusBadRequest, gin.H{"error": "assistant assign not allowed"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "class.assistant.assign", classID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) listGlobalLectures(c *gin.Context) {
	course, ok := a.content.DefaultCourse()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"course": course, "lectures": course.Lectures})
}

func (a *App) getGlobalLecture(c *gin.Context) {
	course, ok := a.content.DefaultCourse()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}
	markdown, err := course.RenderLecture(c.Param("lectureID"))
	if errors.Is(err, os.ErrNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": "lecture not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lecture render failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"markdown": markdown})
}

func (a *App) listLectures(c *gin.Context) {
	course, ok := a.classContent(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"lectures": course.Lectures})
}

func (a *App) getLecture(c *gin.Context) {
	course, ok := a.classContent(c)
	if !ok {
		return
	}
	markdown, err := course.RenderLecture(c.Param("lectureID"))
	if errors.Is(err, os.ErrNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": "lecture not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lecture render failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"markdown": markdown})
}

func (a *App) listAssignments(c *gin.Context) {
	course, ok := a.classContent(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"assignments": course.Assignments})
}

func (a *App) getAssignment(c *gin.Context) {
	course, ok := a.classContent(c)
	if !ok {
		return
	}
	markdown, assignment, err := course.RenderAssignment(c.Param("assignmentID"))
	if errors.Is(err, os.ErrNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": "assignment not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "assignment render failed"})
		return
	}
	status := a.assignmentStatus(c, c.Param("classID"), c.Param("assignmentID"))
	c.JSON(http.StatusOK, gin.H{"assignment": assignment, "markdown": markdown, "status": status})
}

func (a *App) submitAssignment(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	assignmentID := c.Param("assignmentID")
	course, ok := a.classContent(c)
	if !ok {
		return
	}
	assignment, found := course.byAssign[assignmentID]
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "assignment not found"})
		return
	}

	var attempt int
	_ = a.db.QueryRow(c.Request.Context(), `
select count(*) + 1 from submissions where class_id=$1 and assignment_id=$2 and user_id=$3
`, classID, assignmentID, user.ID).Scan(&attempt)

	submissionID := uuid.NewString()
	prefix := fmt.Sprintf("submissions/%s/%s/%s/%s", classID, assignmentID, user.ID, submissionID)
	text := strings.TrimSpace(c.PostForm("text"))
	textObject := ""
	if text != "" {
		textObject = prefix + "/answer.txt"
		if err := a.objects.PutText(c.Request.Context(), textObject, text); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "text upload failed"})
			return
		}
	}

	workspace, ok := a.latestWorkspace(c, classID, assignmentID, user.ID)
	if !ok {
		return
	}
	fileObject := prefix + "/workspace.tar.gz"
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := a.workspaces.Archive(c.Request.Context(), workspaceObjects(workspace), assignmentID, writer)
		_ = writer.CloseWithError(err)
		done <- err
	}()
	uploadErr := a.objects.PutStream(c.Request.Context(), fileObject, "application/gzip", reader)
	if uploadErr != nil {
		_ = reader.CloseWithError(uploadErr)
	}
	archiveErr := <-done
	if archiveErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace archive failed"})
		return
	}
	if uploadErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace upload failed"})
		return
	}

	late := time.Now().After(assignment.Meta.DueAt)
	_, err := a.db.Exec(c.Request.Context(), `
insert into submissions (id, class_id, assignment_id, user_id, text_object, file_object, attempt, late)
values ($1,$2,$3,$4,$5,$6,$7,$8)
`, submissionID, classID, assignmentID, user.ID, textObject, fileObject, attempt, late)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "submission create failed"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "assignment.submit", submissionID)
	c.JSON(http.StatusCreated, gin.H{"id": submissionID, "attempt": attempt, "late": late, "workspaceObject": fileObject})
}

func (a *App) listSubmissions(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	if ok, err := canGradeClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	rows, err := a.db.Query(c.Request.Context(), `
select s.id, s.class_id, s.assignment_id, s.user_id, u.display_name, s.text_object, s.file_object, s.attempt, s.late, s.created_at,
       g.id, g.score, g.comment, g.published_at
from submissions s
join users u on u.id=s.user_id
left join grades g on g.submission_id=s.id
where s.class_id=$1 and s.assignment_id=$2
order by s.created_at desc
`, classID, c.Param("assignmentID"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "submissions query failed"})
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var s Submission
		var displayName string
		var gradeID, comment sql.NullString
		var score sql.NullFloat64
		var publishedAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.ClassID, &s.AssignmentID, &s.UserID, &displayName, &s.TextObject, &s.FileObject, &s.Attempt, &s.Late, &s.CreatedAt, &gradeID, &score, &comment, &publishedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "submissions scan failed"})
			return
		}
		items = append(items, gin.H{"submission": s, "studentName": displayName, "grade": gin.H{"id": nullString(gradeID), "score": nullFloat(score), "comment": nullString(comment), "publishedAt": nullTime(publishedAt)}})
	}
	c.JSON(http.StatusOK, gin.H{"submissions": items})
}

func (a *App) exportGrades(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	assignmentID := c.Param("assignmentID")
	if c.Query("format") != "" && c.Query("format") != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported export format"})
		return
	}
	if ok, err := canGradeClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	rows, err := a.db.Query(c.Request.Context(), `
select u.username, u.display_name, s.id, s.attempt, s.late, s.created_at, g.score, g.comment, g.published_at
from class_members m
join users u on u.id=m.user_id
left join lateral (
  select id, attempt, late, created_at
  from submissions
  where class_id=m.class_id and assignment_id=$2 and user_id=u.id
  order by attempt desc, created_at desc
  limit 1
) s on true
left join grades g on g.submission_id=s.id
where m.class_id=$1 and m.member_role='student'
order by u.username
`, classID, assignmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "grades export query failed"})
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s-grades.csv"`, classID, assignmentID))
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"username", "displayName", "submissionId", "attempt", "late", "submittedAt", "score", "comment", "publishedAt"})
	for rows.Next() {
		var username, displayName string
		var submissionID, comment sql.NullString
		var attempt sql.NullInt64
		var late sql.NullBool
		var submittedAt, publishedAt sql.NullTime
		var score sql.NullFloat64
		if err := rows.Scan(&username, &displayName, &submissionID, &attempt, &late, &submittedAt, &score, &comment, &publishedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "grades export scan failed"})
			return
		}
		_ = writer.Write([]string{
			username,
			displayName,
			nullCSVString(submissionID),
			nullCSVInt(attempt),
			nullCSVBool(late),
			nullCSVTime(submittedAt),
			nullCSVFloat(score),
			nullCSVString(comment),
			nullCSVTime(publishedAt),
		})
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "grades export rows failed"})
		return
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "grades export write failed"})
		return
	}
}

func (a *App) getSubmission(c *gin.Context) {
	item, ok := a.fetchSubmission(c, c.Param("id"))
	if !ok {
		return
	}
	user := currentUser(c)
	if item.UserID != user.ID {
		if allowed, err := canGradeClass(c.Request.Context(), a.db, user, item.ClassID); err != nil || !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"submission": item})
}

func (a *App) previewSubmission(c *gin.Context) {
	item, ok := a.fetchSubmission(c, c.Param("id"))
	if !ok {
		return
	}
	user := currentUser(c)
	if item.UserID != user.ID {
		if allowed, err := canGradeClass(c.Request.Context(), a.db, user, item.ClassID); err != nil || !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	text := ""
	if item.TextObject != "" {
		var err error
		text, err = a.objects.GetText(c.Request.Context(), item.TextObject)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "submission preview failed"})
			return
		}
	}
	fileName := ""
	if item.FileObject != "" {
		fileName = filepath.Base(item.FileObject)
	}
	c.JSON(http.StatusOK, gin.H{"submission": item, "text": text, "fileName": fileName, "fileObject": item.FileObject})
}

func (a *App) downloadSubmissionArchive(c *gin.Context) {
	item, ok := a.fetchSubmission(c, c.Param("id"))
	if !ok {
		return
	}
	user := currentUser(c)
	if item.UserID != user.ID {
		if allowed, err := canGradeClass(c.Request.Context(), a.db, user, item.ClassID); err != nil || !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if item.FileObject == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "archive not found"})
		return
	}
	obj, err := a.objects.Get(c.Request.Context(), item.FileObject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "archive download failed"})
		return
	}
	defer obj.Close()
	stat, err := obj.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "archive stat failed"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s-attempt-%d.tar.gz"`, item.AssignmentID, item.UserID, item.Attempt))
	c.DataFromReader(http.StatusOK, stat.Size, "application/gzip", obj, nil)
}

func (a *App) gradeSubmission(c *gin.Context) {
	user := currentUser(c)
	submission, ok := a.fetchSubmission(c, c.Param("id"))
	if !ok {
		return
	}
	if allowed, err := canGradeClass(c.Request.Context(), a.db, user, submission.ClassID); err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		Score   float64 `json:"score" binding:"gte=0,lte=100"`
		Comment string  `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid grade request"})
		return
	}
	gradeID := uuid.NewString()
	err := a.db.QueryRow(c.Request.Context(), `
insert into grades (id, submission_id, score, comment, grader_id)
values ($1,$2,$3,$4,$5)
on conflict (submission_id) do update set score=excluded.score, comment=excluded.comment, grader_id=excluded.grader_id, updated_at=now()
returning id
`, gradeID, submission.ID, req.Score, req.Comment, user.ID).Scan(&gradeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "grade save failed"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "submission.grade", submission.ID)
	c.JSON(http.StatusOK, gin.H{"id": gradeID})
}

func (a *App) publishGrade(c *gin.Context) {
	user := currentUser(c)
	var classID string
	err := a.db.QueryRow(c.Request.Context(), `
select s.class_id from grades g join submissions s on s.id=g.submission_id where g.id=$1
`, c.Param("id")).Scan(&classID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "grade not found"})
		return
	}
	if allowed, err := canGradeClass(c.Request.Context(), a.db, user, classID); err != nil || !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	_, err = a.db.Exec(c.Request.Context(), `update grades set published_at=now(), updated_at=now() where id=$1`, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "grade publish failed"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "grade.publish", c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) startWorkspace(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	assignmentID := c.Param("assignmentID")
	course, ok := a.classContent(c)
	if !ok {
		return
	}
	assignment, found := course.byAssign[assignmentID]
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "assignment not found"})
		return
	}

	if existing, found, err := a.findWorkspace(c.Request.Context(), classID, assignmentID, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace query failed"})
		return
	} else if found {
		objects := workspaceObjects(existing)
		if a.workspaces.Exists(c.Request.Context(), objects) {
			c.JSON(http.StatusOK, gin.H{"workspace": gin.H{"id": existing.ID, "ideURL": "/ide/s/" + existing.ID + "/", "status": existing.Status}})
			return
		}
		a.workspaces.Stop(c.Request.Context(), objects)
		_, _ = a.db.Exec(c.Request.Context(), `update workspaces set status='stopped' where id=$1`, existing.ID)
	}

	id := uuid.NewString()
	objects, err := a.workspaces.Create(c.Request.Context(), id, user.ID, course.ID, assignmentID, assignment.Meta.RStudioImage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace create failed"})
		return
	}
	ideURL := "/ide/s/" + id + "/"
	_, err = a.db.Exec(c.Request.Context(), `
insert into workspaces (id, user_id, class_id, assignment_id, pod_name, service_name, pvc_name, status)
values ($1,$2,$3,$4,$5,$6,$7,'creating')
`, id, user.ID, classID, assignmentID, objects.PodName, objects.ServiceName, objects.PVCName)
	if err != nil {
		a.workspaces.Stop(c.Request.Context(), objects)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace record failed"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "workspace.start", id)
	c.JSON(http.StatusCreated, gin.H{"workspace": gin.H{"id": id, "ideURL": ideURL, "status": "creating"}})
}

func (a *App) startSubmissionWorkspace(c *gin.Context) {
	user := currentUser(c)
	submission, ok := a.fetchSubmission(c, c.Param("id"))
	if !ok {
		return
	}
	if submission.UserID != user.ID {
		if allowed, err := canGradeClass(c.Request.Context(), a.db, user, submission.ClassID); err != nil || !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	if submission.FileObject == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace archive not found"})
		return
	}
	courseID, err := classCourse(c.Request.Context(), a.db, submission.ClassID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "class not found"})
		return
	}
	course, found := a.content.Course(courseID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}
	assignment, found := course.byAssign[submission.AssignmentID]
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "assignment not found"})
		return
	}

	workspace, created, ok := a.reviewWorkspace(c, user, submission, course.ID, assignment.Meta.RStudioImage)
	if !ok {
		return
	}
	objects := workspaceObjects(workspace)
	archive, err := a.objects.Get(c.Request.Context(), submission.FileObject)
	if err != nil {
		if created {
			a.workspaces.Stop(c.Request.Context(), objects)
			_, _ = a.db.Exec(c.Request.Context(), `update workspaces set status='stopped' where id=$1`, workspace.ID)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace archive download failed"})
		return
	}
	defer archive.Close()
	if err := a.workspaces.Restore(c.Request.Context(), objects, submission.AssignmentID, archive); err != nil {
		if created {
			a.workspaces.Stop(c.Request.Context(), objects)
			_, _ = a.db.Exec(c.Request.Context(), `update workspaces set status='stopped' where id=$1`, workspace.ID)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace restore failed"})
		return
	}
	_, _ = a.db.Exec(c.Request.Context(), `update workspaces set status='running', last_seen_at=now() where id=$1`, workspace.ID)
	logEvent(c.Request.Context(), a.db, user.ID, "workspace.review.start", submission.ID)
	c.JSON(http.StatusCreated, gin.H{"workspace": gin.H{"id": workspace.ID, "ideURL": "/ide/s/" + workspace.ID + "/", "status": "running"}})
}

func (a *App) heartbeatWorkspace(c *gin.Context) {
	user := currentUser(c)
	result, err := a.db.Exec(c.Request.Context(), `
update workspaces set last_seen_at=now() where id=$1 and (user_id=$2 or $3)
`, c.Param("id"), user.ID, isAdmin(user))
	if err != nil || result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) stopWorkspace(c *gin.Context) {
	user := currentUser(c)
	var objects WorkspaceObjects
	err := a.db.QueryRow(c.Request.Context(), `
select pod_name, service_name, pvc_name from workspaces where id=$1 and (user_id=$2 or $3)
`, c.Param("id"), user.ID, isAdmin(user)).Scan(&objects.PodName, &objects.ServiceName, &objects.PVCName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}
	a.workspaces.Stop(c.Request.Context(), objects)
	_, _ = a.db.Exec(c.Request.Context(), `update workspaces set status='stopped' where id=$1`, c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) listUsers(c *gin.Context) {
	if !isAdmin(currentUser(c)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	rows, err := a.db.Query(c.Request.Context(), `select id, username, display_name, role, status, created_at from users order by created_at desc`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "users query failed"})
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var item User
		if err := rows.Scan(&item.ID, &item.Username, &item.DisplayName, &item.Role, &item.Status, &item.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "users scan failed"})
			return
		}
		users = append(users, item)
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (a *App) createUser(c *gin.Context) {
	actor := currentUser(c)
	if !isAdmin(actor) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		Username    string `json:"username" binding:"required"`
		DisplayName string `json:"displayName" binding:"required"`
		Password    string `json:"password" binding:"required"`
		Role        Role   `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user request"})
		return
	}
	if !canAssignGlobalRole(actor, req.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password hash failed"})
		return
	}
	id := uuid.NewString()
	_, err = a.db.Exec(c.Request.Context(), `
insert into users (id, username, display_name, password_hash, role, status) values ($1,$2,$3,$4,$5,'active')
`, id, req.Username, req.DisplayName, hash, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user create failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (a *App) bulkUsers(c *gin.Context) {
	actor := currentUser(c)
	if !isAdmin(actor) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		IDs    []string `json:"ids" binding:"required"`
		Action string   `json:"action" binding:"required"`
		Role   Role     `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user bulk request"})
		return
	}
	if req.Action == "setRole" && !canAssignGlobalRole(actor, req.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	updated := 0
	for _, id := range uniqueStrings(req.IDs) {
		target, ok, err := a.fetchUser(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user query failed"})
			return
		}
		if !ok {
			continue
		}
		if !canManageTargetUser(actor, target) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		switch req.Action {
		case "disable":
			if target.ID == actor.ID {
				c.JSON(http.StatusBadRequest, gin.H{"error": "cannot disable current user"})
				return
			}
			if _, err := a.db.Exec(c.Request.Context(), `update users set status='disabled' where id=$1`, target.ID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "user bulk update failed"})
				return
			}
			_, _ = a.db.Exec(c.Request.Context(), `delete from sessions where user_id=$1`, target.ID)
		case "activate":
			if _, err := a.db.Exec(c.Request.Context(), `update users set status='active' where id=$1`, target.ID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "user bulk update failed"})
				return
			}
		case "setRole":
			if target.ID == actor.ID {
				c.JSON(http.StatusBadRequest, gin.H{"error": "cannot change current user role"})
				return
			}
			if _, err := a.db.Exec(c.Request.Context(), `update users set role=$1 where id=$2`, req.Role, target.ID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "user bulk update failed"})
				return
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user bulk action"})
			return
		}
		updated++
	}
	logEvent(c.Request.Context(), a.db, actor.ID, "users.bulk", req.Action)
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

func (a *App) updateUser(c *gin.Context) {
	actor := currentUser(c)
	if !isAdmin(actor) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	target, ok, err := a.fetchUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user query failed"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !canManageTargetUser(actor, target) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		DisplayName *string `json:"displayName"`
		Role        *Role   `json:"role"`
		Status      *string `json:"status"`
		Password    *string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user update"})
		return
	}
	if req.DisplayName != nil {
		if _, err := a.db.Exec(c.Request.Context(), `update users set display_name=$1 where id=$2`, strings.TrimSpace(*req.DisplayName), target.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user update failed"})
			return
		}
	}
	if req.Role != nil {
		if target.ID == actor.ID && *req.Role != target.Role {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot change current user role"})
			return
		}
		if !canAssignGlobalRole(actor, *req.Role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if _, err := a.db.Exec(c.Request.Context(), `update users set role=$1 where id=$2`, *req.Role, target.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user update failed"})
			return
		}
	}
	if req.Status != nil {
		if *req.Status != "active" && *req.Status != "disabled" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		if target.ID == actor.ID && *req.Status == "disabled" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot disable current user"})
			return
		}
		if _, err := a.db.Exec(c.Request.Context(), `update users set status=$1 where id=$2`, *req.Status, target.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user update failed"})
			return
		}
		if *req.Status == "disabled" {
			_, _ = a.db.Exec(c.Request.Context(), `delete from sessions where user_id=$1`, target.ID)
		}
	}
	if req.Password != nil {
		if err := a.updateUserPassword(c.Request.Context(), target.ID, *req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "password update failed"})
			return
		}
	}
	logEvent(c.Request.Context(), a.db, actor.ID, "user.update", target.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) deleteUser(c *gin.Context) {
	actor := currentUser(c)
	if !isAdmin(actor) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	target, ok, err := a.fetchUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user query failed"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if target.ID == actor.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot disable current user"})
		return
	}
	if !canManageTargetUser(actor, target) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if _, err := a.db.Exec(c.Request.Context(), `update users set status='disabled' where id=$1`, target.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user disable failed"})
		return
	}
	_, _ = a.db.Exec(c.Request.Context(), `delete from sessions where user_id=$1`, target.ID)
	logEvent(c.Request.Context(), a.db, actor.ID, "user.disable", target.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) resetUserPassword(c *gin.Context) {
	actor := currentUser(c)
	if !isAdmin(actor) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	target, ok, err := a.fetchUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user query failed"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !canManageTargetUser(actor, target) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Password) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password request"})
		return
	}
	if err := a.updateUserPassword(c.Request.Context(), target.ID, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password reset failed"})
		return
	}
	logEvent(c.Request.Context(), a.db, actor.ID, "user.password.reset", target.ID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) importCourse(c *gin.Context) {
	if !isAdmin(currentUser(c)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	courseID := strings.TrimSpace(c.PostForm("courseId"))
	file, err := c.FormFile("file")
	if courseID == "" || err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "courseId and file are required"})
		return
	}
	tmp, cleanup, err := saveUploadedZip(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "course upload failed"})
		return
	}
	defer cleanup()
	dest := filepath.Join(a.cfg.ContentRoot, "courses", courseID)
	if err := importCourseZip(tmp, dest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "course import failed"})
		return
	}
	if err := a.content.Reload(a.cfg.WorkspaceImageDefault); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "course reload failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) reloadCourses(c *gin.Context) {
	if !isAdmin(currentUser(c)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if err := a.content.Reload(a.cfg.WorkspaceImageDefault); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "warning": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) classContent(c *gin.Context) (*CourseBundle, bool) {
	classID := c.Param("classID")
	if ok, err := canAccessClass(c.Request.Context(), a.db, currentUser(c), classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return nil, false
	}
	courseID, err := classCourse(c.Request.Context(), a.db, classID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "class not found"})
		return nil, false
	}
	course, ok := a.content.Course(courseID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return nil, false
	}
	return course, true
}

func (a *App) assignmentStatus(c *gin.Context, classID, assignmentID string) gin.H {
	user := currentUser(c)
	var latestID sql.NullString
	var latestAt sql.NullTime
	var attempts int
	_ = a.db.QueryRow(c.Request.Context(), `
select latest.id, latest.created_at, stats.attempts
from (select count(*) attempts from submissions where class_id=$1 and assignment_id=$2 and user_id=$3) stats
left join lateral (
  select id, created_at from submissions where class_id=$1 and assignment_id=$2 and user_id=$3 order by created_at desc limit 1
) latest on true
`, classID, assignmentID, user.ID).Scan(&latestID, &latestAt, &attempts)

	var gradeID, comment sql.NullString
	var score sql.NullFloat64
	var publishedAt sql.NullTime
	if latestID.Valid {
		_ = a.db.QueryRow(c.Request.Context(), `
select id, score, comment, published_at from grades where submission_id=$1
`, latestID.String).Scan(&gradeID, &score, &comment, &publishedAt)
	}
	return gin.H{
		"latestSubmissionId": nullString(latestID),
		"latestSubmittedAt":  nullTime(latestAt),
		"attempts":           attempts,
		"grade": gin.H{
			"id":          nullString(gradeID),
			"score":       nullFloat(score),
			"comment":     nullString(comment),
			"publishedAt": nullTime(publishedAt),
		},
	}
}

func (a *App) fetchSubmission(c *gin.Context, id string) (Submission, bool) {
	var item Submission
	err := a.db.QueryRow(c.Request.Context(), `
select id, class_id, assignment_id, user_id, text_object, file_object, attempt, late, created_at
from submissions where id=$1
`, id).Scan(&item.ID, &item.ClassID, &item.AssignmentID, &item.UserID, &item.TextObject, &item.FileObject, &item.Attempt, &item.Late, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return Submission{}, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "submission query failed"})
		return Submission{}, false
	}
	return item, true
}

func (a *App) latestWorkspace(c *gin.Context, classID, assignmentID, userID string) (Workspace, bool) {
	workspace, found, err := a.findWorkspace(c.Request.Context(), classID, assignmentID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace query failed"})
		return Workspace{}, false
	}
	if !found || !a.workspaces.Exists(c.Request.Context(), workspaceObjects(workspace)) {
		if found {
			a.workspaces.Stop(c.Request.Context(), workspaceObjects(workspace))
			_, _ = a.db.Exec(c.Request.Context(), `update workspaces set status='stopped' where id=$1`, workspace.ID)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace not started"})
		return Workspace{}, false
	}
	if err := a.workspaces.WaitReady(c.Request.Context(), workspaceObjects(workspace), 60*time.Second); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace not ready"})
		return Workspace{}, false
	}
	return workspace, true
}

func (a *App) reviewWorkspace(c *gin.Context, user User, submission Submission, courseID, image string) (Workspace, bool, bool) {
	if existing, found, err := a.findWorkspace(c.Request.Context(), submission.ClassID, submission.AssignmentID, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace query failed"})
		return Workspace{}, false, false
	} else if found {
		objects := workspaceObjects(existing)
		if a.workspaces.Exists(c.Request.Context(), objects) {
			if err := a.workspaces.WaitReady(c.Request.Context(), objects, 60*time.Second); err != nil {
				a.workspaces.Stop(c.Request.Context(), objects)
				_, _ = a.db.Exec(c.Request.Context(), `update workspaces set status='stopped' where id=$1`, existing.ID)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace not ready"})
				return Workspace{}, false, false
			}
			return existing, false, true
		}
		a.workspaces.Stop(c.Request.Context(), objects)
		_, _ = a.db.Exec(c.Request.Context(), `update workspaces set status='stopped' where id=$1`, existing.ID)
	}

	id := uuid.NewString()
	objects, err := a.workspaces.Create(c.Request.Context(), id, user.ID, courseID, submission.AssignmentID, image)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace create failed"})
		return Workspace{}, false, false
	}
	if err := a.workspaces.WaitReady(c.Request.Context(), objects, 60*time.Second); err != nil {
		a.workspaces.Stop(c.Request.Context(), objects)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace not ready"})
		return Workspace{}, true, false
	}
	_, err = a.db.Exec(c.Request.Context(), `
insert into workspaces (id, user_id, class_id, assignment_id, pod_name, service_name, pvc_name, status)
values ($1,$2,$3,$4,$5,$6,$7,'running')
`, id, user.ID, submission.ClassID, submission.AssignmentID, objects.PodName, objects.ServiceName, objects.PVCName)
	if err != nil {
		a.workspaces.Stop(c.Request.Context(), objects)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "workspace record failed"})
		return Workspace{}, true, false
	}
	return Workspace{ID: id, UserID: user.ID, ClassID: submission.ClassID, AssignmentID: submission.AssignmentID, PodName: objects.PodName, ServiceName: objects.ServiceName, PVCName: objects.PVCName, Status: "running"}, true, true
}

func (a *App) findWorkspace(ctx context.Context, classID, assignmentID, userID string) (Workspace, bool, error) {
	var item Workspace
	err := a.db.QueryRow(ctx, `
select id, user_id, class_id, assignment_id, pod_name, service_name, pvc_name, status, last_seen_at, created_at
from workspaces
where class_id=$1 and assignment_id=$2 and user_id=$3 and status <> 'stopped'
order by created_at desc
limit 1
`, classID, assignmentID, userID).Scan(&item.ID, &item.UserID, &item.ClassID, &item.AssignmentID, &item.PodName, &item.ServiceName, &item.PVCName, &item.Status, &item.LastSeenAt, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, false, nil
	}
	return item, err == nil, err
}

func workspaceObjects(workspace Workspace) WorkspaceObjects {
	return WorkspaceObjects{PodName: workspace.PodName, ServiceName: workspace.ServiceName, PVCName: workspace.PVCName}
}

func (a *App) deleteClassByID(ctx context.Context, classID string) (bool, error) {
	rows, err := a.db.Query(ctx, `
select pod_name, service_name, pvc_name
from workspaces
where class_id=$1 and status <> 'stopped'
`, classID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var objects WorkspaceObjects
		if err := rows.Scan(&objects.PodName, &objects.ServiceName, &objects.PVCName); err != nil {
			return false, err
		}
		a.workspaces.Stop(ctx, objects)
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	cmd, err := a.db.Exec(ctx, `delete from classes where id=$1`, classID)
	if err != nil {
		return false, err
	}
	return cmd.RowsAffected() > 0, nil
}

func (a *App) removeClassMember(ctx context.Context, classID, userID string) (bool, error) {
	cmd, err := a.db.Exec(ctx, `delete from class_members where class_id=$1 and user_id=$2`, classID, userID)
	if err != nil {
		return false, err
	}
	// 助教角色仅通过 class_members 管理，不修改全局角色
	return cmd.RowsAffected() > 0, nil
}

func (a *App) setClassMemberRole(ctx context.Context, classID, userID, memberRole string) (bool, error) {
	var target User
	err := a.db.QueryRow(ctx, `
select u.id, u.username, u.display_name, u.role, u.status, u.created_at
from class_members m join users u on u.id=m.user_id
where m.class_id=$1 and m.user_id=$2
`, classID, userID).Scan(&target.ID, &target.Username, &target.DisplayName, &target.Role, &target.Status, &target.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if target.Role != RoleStudent && target.Role != RoleAssistant {
		return false, nil
	}
	// 助教仅是班级成员标签，不修改全局角色
	if _, err := a.db.Exec(ctx, `update class_members set member_role=$1 where class_id=$2 and user_id=$3`, memberRole, classID, userID); err != nil {
		return false, err
	}
	return true, nil
}


func (a *App) fetchUser(ctx context.Context, id string) (User, bool, error) {
	var item User
	err := a.db.QueryRow(ctx, `
select id, username, display_name, role, status, created_at from users where id=$1
`, id).Scan(&item.ID, &item.Username, &item.DisplayName, &item.Role, &item.Status, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, false, nil
	}
	return item, err == nil, err
}

func (a *App) updateUserPassword(ctx context.Context, userID, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `update users set password_hash=$1 where id=$2`, hash, userID); err != nil {
		return err
	}
	_, _ = a.db.Exec(ctx, `delete from sessions where user_id=$1`, userID)
	return nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func nullCSVString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullCSVFloat(value sql.NullFloat64) string {
	if value.Valid {
		return strconv.FormatFloat(value.Float64, 'f', -1, 64)
	}
	return ""
}

func nullCSVInt(value sql.NullInt64) string {
	if value.Valid {
		return strconv.FormatInt(value.Int64, 10)
	}
	return ""
}

func nullCSVBool(value sql.NullBool) string {
	if value.Valid {
		return strconv.FormatBool(value.Bool)
	}
	return ""
}

func nullCSVTime(value sql.NullTime) string {
	if value.Valid {
		return value.Time.Format(time.RFC3339)
	}
	return ""
}

func parseCSVLine(line string) []string {
	parts := strings.Split(line, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func atoiDefault(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

type studentImport struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func parseStudentImport(c *gin.Context) ([]studentImport, error) {
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		file, err := c.FormFile("file")
		if err != nil {
			return nil, err
		}
		src, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer src.Close()
		return readStudentsCSV(src)
	}
	var req struct {
		Students []studentImport `json:"students" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	return req.Students, nil
}

func readStudentsCSV(src io.Reader) ([]studentImport, error) {
	records, err := csv.NewReader(src).ReadAll()
	if err != nil {
		return nil, err
	}
	students := make([]studentImport, 0, len(records))
	for index, record := range records {
		if index == 0 && len(record) >= 3 && strings.EqualFold(strings.TrimSpace(record[0]), "username") {
			continue
		}
		if len(record) < 3 {
			return nil, fmt.Errorf("csv row %d needs username, displayName, password", index+1)
		}
		students = append(students, studentImport{
			Username:    strings.TrimSpace(record[0]),
			DisplayName: strings.TrimSpace(record[1]),
			Password:    strings.TrimSpace(record[2]),
		})
	}
	return students, nil
}

// ── Class Teacher Management ──────────────────────────────────

func (a *App) listClassTeachers(c *gin.Context) {
	classID := c.Param("classID")
	rows, err := a.db.Query(c.Request.Context(), `
select ct.user_id, u.username, u.display_name, u.role
from class_teachers ct join users u on u.id = ct.user_id
where ct.class_id = $1
order by u.display_name
`, classID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()
	type teacherRow struct {
		UserID      string `json:"userId"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		GlobalRole  string `json:"globalRole"`
	}
	var teachers []teacherRow
	for rows.Next() {
		var t teacherRow
		if err := rows.Scan(&t.UserID, &t.Username, &t.DisplayName, &t.GlobalRole); err != nil {
			continue
		}
		teachers = append(teachers, t)
	}
	c.JSON(http.StatusOK, gin.H{"teachers": teachers})
}

func (a *App) addClassTeacher(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		UserID string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	_, err := a.db.Exec(c.Request.Context(), `
insert into class_teachers (class_id, user_id) values ($1,$2) on conflict do nothing
`, classID, req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加教师失败"})
		return
	}
	logEvent(c.Request.Context(), a.db, user.ID, "class.add_teacher", classID+":"+req.UserID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) removeClassTeacher(c *gin.Context) {
	user := currentUser(c)
	classID := c.Param("classID")
	targetID := c.Param("userID")
	if ok, err := canManageClass(c.Request.Context(), a.db, user, classID); err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	_, _ = a.db.Exec(c.Request.Context(), `delete from class_teachers where class_id=$1 and user_id=$2`, classID, targetID)
	logEvent(c.Request.Context(), a.db, user.ID, "class.remove_teacher", classID+":"+targetID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ── Course Management ─────────────────────────────────────────

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
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Code        string    `json:"code"`
		Description string    `json:"description"`
		Status      string    `json:"status"`
		ContentRoot string    `json:"contentRoot"`
		CreatedBy   string    `json:"createdBy"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
		MemberCount int       `json:"memberCount"`
		ClassCount  int       `json:"classCount"`
	}
	var courses []courseRow
	for rows.Next() {
		var r courseRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Code, &r.Description, &r.Status, &r.ContentRoot, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &r.MemberCount, &r.ClassCount); err != nil {
			continue
		}
		courses = append(courses, r)
	}
	c.JSON(http.StatusOK, gin.H{"courses": courses})
}

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
		c.JSON(http.StatusConflict, gin.H{"error": "课程创建失败，code 可能重复"})
		return
	}
	_, _ = a.db.Exec(c.Request.Context(), `
insert into course_members (course_id, user_id, member_role, invited_by)
values ($1,$2,'admin',$3)
on conflict do nothing
`, id, user.ID, user.ID)
	logEvent(c.Request.Context(), a.db, user.ID, "course.create", id)
	c.JSON(http.StatusCreated, gin.H{"id": id})
}

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
		UserID     string    `json:"userId"`
		Username   string    `json:"username"`
		GlobalRole string    `json:"globalRole"`
		MemberRole string    `json:"memberRole"`
		JoinedAt   time.Time `json:"joinedAt"`
		InvitedBy  *string   `json:"invitedBy"`
	}
	var members []memberRow
	for rows.Next() {
		var m memberRow
		if err := rows.Scan(&m.UserID, &m.Username, &m.GlobalRole, &m.MemberRole, &m.JoinedAt, &m.InvitedBy); err != nil {
			continue
		}
		members = append(members, m)
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

func (a *App) addCourseMember(c *gin.Context) {
	user := currentUser(c)
	courseID := c.Param("courseID")
	if !a.isCourseAdmin(c.Request.Context(), user.ID, courseID) && user.Role != RoleRoot {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	var req struct {
		UserID string `json:"userId" binding:"required"`
		Role   string `json:"role" binding:"required"`
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

func nullString(value sql.NullString) any {
	if value.Valid {
		return value.String
	}
	return nil
}

func nullFloat(value sql.NullFloat64) any {
	if value.Valid {
		return value.Float64
	}
	return nil
}

func nullTime(value sql.NullTime) any {
	if value.Valid {
		return value.Time
	}
	return nil
}
