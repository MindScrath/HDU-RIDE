import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  async rewrites() {
    const goBackend = process.env.NEXT_PUBLIC_GO_API_URL ?? 'http://localhost:8080'
    return [
      {
        source: '/api/:path*',
        destination: `${goBackend}/api/:path*`,
      },
      {
        source: '/ide/:path*',
        destination: `${goBackend}/ide/:path*`,
      },
    ]
  },
}

export default nextConfig
