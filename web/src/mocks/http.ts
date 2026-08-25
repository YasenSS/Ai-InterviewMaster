import { mockClock } from "./clock";

export function json(data: unknown, status = 200) {
  return Response.json(data, {
    status,
    headers: { "x-request-id": requestId() },
  });
}

export function noContent() {
  return new Response(null, { status: 204, headers: { "x-request-id": requestId() } });
}

export function apiError(status: number, code: string, message: string, fields?: Record<string, string[]>) {
  return json(
    {
      code,
      message,
      request_id: requestId(),
      details: fields ? { fields } : undefined,
    },
    status,
  );
}

function requestId() {
  return `mock-${mockClock.now()}`;
}
