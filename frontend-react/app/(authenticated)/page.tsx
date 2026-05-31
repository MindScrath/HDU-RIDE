// app/(authenticated)/page.tsx
import { redirect } from 'next/navigation'

export default function Home() {
  redirect('/classes')
}
