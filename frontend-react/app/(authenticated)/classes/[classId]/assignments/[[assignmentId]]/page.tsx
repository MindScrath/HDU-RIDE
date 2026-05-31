import { AssignmentViewer } from '@/components/assignments/AssignmentViewer'

export default async function ClassAssignmentPage({
  params,
}: {
  params: Promise<{ classId: string; assignmentId?: string[] }>
}) {
  const { classId, assignmentId } = await params
  return <AssignmentViewer classId={classId} assignmentId={assignmentId?.[0]} />
}
