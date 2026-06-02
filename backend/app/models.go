package app

import "time"

type Role string

const (
	RoleRoot      Role = "root"
	RoleAdmin     Role = "admin"
	RoleTeacher   Role = "teacher"
	RoleAssistant Role = "assistant"
	RoleStudent   Role = "student"
)

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Role        Role      `json:"role"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Class struct {
	ID        string    `json:"id"`
	CourseID  string    `json:"courseId"`
	Name      string    `json:"name"`
	Term      string    `json:"term"`
	Note      string    `json:"note"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

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

type Submission struct {
	ID           string    `json:"id"`
	ClassID      string    `json:"classId"`
	AssignmentID string    `json:"assignmentId"`
	UserID       string    `json:"userId"`
	TextObject   string    `json:"textObject"`
	FileObject   string    `json:"fileObject"`
	Attempt      int       `json:"attempt"`
	Late         bool      `json:"late"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Grade struct {
	ID           string     `json:"id"`
	SubmissionID string     `json:"submissionId"`
	Score        float64    `json:"score"`
	Comment      string     `json:"comment"`
	GraderID     string     `json:"graderId"`
	PublishedAt  *time.Time `json:"publishedAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type Workspace struct {
	ID           string    `json:"id"`
	UserID       string    `json:"userId"`
	ClassID      string    `json:"classId"`
	AssignmentID string    `json:"assignmentId"`
	PodName      string    `json:"podName"`
	ServiceName  string    `json:"serviceName"`
	PVCName      string    `json:"pvcName"`
	Status       string    `json:"status"`
	IDEURL       string    `json:"ideURL"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Event struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	CreatedAt time.Time `json:"createdAt"`
}
