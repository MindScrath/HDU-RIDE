import { AssignmentViewer } from '@/components/assignments/AssignmentViewer'

export default async function GlobalAssignmentPage({
  params,
  searchParams,
}: {
  params: Promise<{ assignmentId: string[] }>
  searchParams: Promise<{ classId?: string }>
}) {
  const { assignmentId } = await params
  const { classId } = await searchParams
  return <AssignmentViewer assignmentId={assignmentId?.[0]} classId={classId} />
}
