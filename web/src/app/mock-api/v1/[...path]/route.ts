import { type NextRequest } from "next/server";

import { apiError } from "@/mocks/http";
import { handleMockApi } from "@/mocks/router";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

async function route(request: NextRequest, context: { params: Promise<{ path: string[] }> }) {
  if (process.env.MOCK_API !== "1" && process.env.NEXT_PUBLIC_MOCK_API !== "1") {
    return apiError(404, "NOT_FOUND", "当前不是 Mock 环境。");
  }
  const { path } = await context.params;
  return handleMockApi(request, path ?? []);
}

export const GET = route;
export const POST = route;
export const PUT = route;
export const PATCH = route;
export const DELETE = route;
export const OPTIONS = route;
