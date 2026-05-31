// proxy.ts — Next.js 16 replaces middleware.ts
import { NextResponse } from 'next/server'
import type { NextRequest } from 'next/server'

export function proxy(request: NextRequest) {
  const sessionToken = request.cookies.get('session_token')?.value
  const { pathname } = request.nextUrl

  // Allow API routes to pass through (handled by Go backend or Next.js API routes)
  if (pathname.startsWith('/api/')) {
    return NextResponse.next()
  }

  // Not logged in → redirect to /login (except /login itself)
  if (!sessionToken && pathname !== '/login') {
    return NextResponse.redirect(new URL('/login', request.url))
  }

  // Logged in and on /login → redirect to /classes
  if (sessionToken && pathname === '/login') {
    return NextResponse.redirect(new URL('/classes', request.url))
  }

  return NextResponse.next()
}

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
}
