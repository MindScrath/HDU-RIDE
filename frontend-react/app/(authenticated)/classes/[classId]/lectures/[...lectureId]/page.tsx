import { LectureViewer } from '@/components/lectures/LectureViewer'

export default async function ClassLecturePage({
  params,
}: {
  params: Promise<{ classId: string; lectureId: string[] }>
}) {
  const { classId, lectureId } = await params
  return <LectureViewer classId={classId} lectureId={lectureId?.[0]} />
}
