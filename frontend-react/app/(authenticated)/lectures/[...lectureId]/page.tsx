import { LectureViewer } from '@/components/lectures/LectureViewer'

export default async function GlobalLecturePage({
  params,
}: {
  params: Promise<{ lectureId: string[] }>
}) {
  const { lectureId } = await params
  return <LectureViewer lectureId={lectureId?.[0]} />
}
