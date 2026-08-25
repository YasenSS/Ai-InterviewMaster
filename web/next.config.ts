import type { NextConfig } from "next";

const mockApi = process.env.MOCK_API === "1" || process.env.NEXT_PUBLIC_MOCK_API === "1";
const apiBaseUrl = process.env.API_INTERNAL_BASE_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  async rewrites() {
    if (mockApi) {
      return [{ source: "/api/v1/:path*", destination: "/mock-api/v1/:path*" }];
    }
    return [
      {
        source: "/api/v1/:path*",
        destination: `${apiBaseUrl}/api/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
