import Link from 'next/link'

export default function NotFound() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-[#f7f9fc]">
      <div className="text-center">
        <h1 className="text-6xl font-bold text-[#0b5ed7] mb-4">404</h1>
        <h2 className="text-xl text-[#1f2937] mb-2">页面未找到</h2>
        <p className="text-[#6b7280] mb-6">请检查地址是否正确，或返回首页</p>
        <Link href="/classes" className="text-[#0b5ed7] hover:underline font-medium">
          返回班级列表
        </Link>
      </div>
    </div>
  )
}
