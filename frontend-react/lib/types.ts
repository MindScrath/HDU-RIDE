export type Role = 'root' | 'admin' | 'teacher' | 'assistant' | 'student'

export interface User {
  id: string
  username: string
  displayName: string
  role: Role
  status: string
  createdAt?: string
}

export interface ClassItem {
  id: string
  courseId: string
  name: string
  term: string
  note: string
  createdBy: string
}

export interface Lecture {
  id: string
  file: string
  title: string
  order: number
}

export interface LectureChapter {
  id: string
  title: string
  order: number
  sections: Lecture[]
}

export interface Assignment {
  id: string
  title: string
  openAt: string
  dueAt: string
  rstudioImage: string
  starter: string
  submitPath: string
}

export interface Submission {
  id: string
  classId: string
  assignmentId: string
  userId: string
  textObject: string
  fileObject: string
  attempt: number
  late: boolean
  createdAt: string
}

export interface SubmissionRow {
  submission: Submission
  studentName: string
  grade: {
    id: string | null
    score: number | null
    comment: string
    publishedAt?: string | null
  }
}

export interface MemberRow {
  user: User
  memberRole: 'student' | 'assistant'
  joinedAt: string
}
