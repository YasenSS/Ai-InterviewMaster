export type AppError = {
  code: string;
  message: string;
  status?: number;
  requestId?: string;
  fieldErrors?: Record<string, string[]>;
  details?: unknown;
};

type ErrorPayload = {
  code?: string;
  message?: string;
  request_id?: string;
  details?: { fields?: Record<string, string[]>; [key: string]: unknown };
};

const TOKEN_KEY = "im_token";
let refreshPromise: Promise<boolean> | null = null;

export const authStorage = {
  get: () => (typeof window === "undefined" ? null : localStorage.getItem(TOKEN_KEY)),
  set: (token: string) => {
    localStorage.setItem(TOKEN_KEY, token);
    document.cookie = "im_session_hint=1; Path=/; SameSite=Lax";
    window.dispatchEvent(new Event("im-auth"));
  },
  clear: () => {
    localStorage.removeItem(TOKEN_KEY);
    document.cookie = "im_session_hint=; Path=/; Max-Age=0; SameSite=Lax";
    window.dispatchEvent(new Event("im-auth"));
  },
};

const friendlyMessage: Record<number, string> = {
  400: "提交的信息有误，请检查后重试。",
  401: "登录状态已失效，请重新登录。",
  403: "你没有权限访问此内容。",
  404: "请求的内容不存在或已被删除。",
  409: "当前状态无法完成该操作，请刷新后重试。",
  429: "操作过于频繁，请稍后再试。",
  500: "服务暂时出现问题，请稍后重试。",
  503: "服务暂时不可用，请稍后重试。",
};

export function normalizeError(error: unknown): AppError {
  if (typeof error === "object" && error !== null && "code" in error && "message" in error) {
    return error as AppError;
  }
  if (error instanceof DOMException && error.name === "AbortError") {
    return { code: "REQUEST_ABORTED", message: "请求已取消。" };
  }
  return { code: "NETWORK_ERROR", message: "网络连接失败，请检查网络后重试。" };
}

async function tryRefresh() {
  if (!refreshPromise) {
    refreshPromise = fetch("/api/v1/auth/refresh", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    })
      .then(async (response) => {
        if (!response.ok) return false;
        const body = (await response.json()) as { access_token?: string };
        if (!body.access_token) return false;
        authStorage.set(body.access_token);
        return true;
      })
      .catch(() => false)
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

export async function apiRequest<T>(
  path: string,
  options: RequestInit & { timeoutMs?: number; skipRefresh?: boolean } = {},
): Promise<T> {
  const { timeoutMs = 15_000, skipRefresh = false, signal, ...requestOptions } = options;
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), timeoutMs);
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  const token = authStorage.get();

  try {
    const response = await fetch(path, {
      ...requestOptions,
      credentials: "include",
      signal: controller.signal,
      headers: {
        "Content-Type": "application/json",
        ...requestOptions.headers,
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
    });

    if (response.status === 401 && !skipRefresh && !path.includes("/auth/")) {
      if (await tryRefresh()) return apiRequest<T>(path, { ...options, skipRefresh: true });
      authStorage.clear();
      if (typeof window !== "undefined") {
        const returnTo = `${window.location.pathname}${window.location.search}`;
        window.location.assign(`/login?return_to=${encodeURIComponent(returnTo)}`);
      }
    }

    const requestId = response.headers.get("x-request-id") ?? undefined;
    if (!response.ok) {
      let payload: ErrorPayload = {};
      try {
        payload = (await response.json()) as ErrorPayload;
      } catch {
        // Non-JSON failures are normalized below.
      }
      throw {
        code: payload.code ?? `HTTP_${response.status}`,
        message: friendlyMessage[response.status] ?? payload.message ?? "请求失败，请稍后重试。",
        status: response.status,
        requestId: payload.request_id ?? requestId,
        fieldErrors: payload.details?.fields,
        details: payload.details,
      } satisfies AppError;
    }

    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
  } catch (error) {
    throw normalizeError(error);
  } finally {
    window.clearTimeout(timeout);
    signal?.removeEventListener("abort", abort);
  }
}

export function uploadFile(
  url: string,
  file: File,
  headers: Record<string, string>,
  onProgress: (progress: number) => void,
) {
  return new Promise<void>((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open("PUT", url);
    Object.entries(headers).forEach(([key, value]) => request.setRequestHeader(key, value));
    request.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable) onProgress(Math.round((event.loaded / event.total) * 100));
    });
    request.addEventListener("load", () => {
      if (request.status >= 200 && request.status < 300) resolve();
      else reject({ code: "UPLOAD_FAILED", message: "文件上传失败，请重试。" } satisfies AppError);
    });
    request.addEventListener("error", () =>
      reject({ code: "NETWORK_ERROR", message: "上传连接中断，请检查网络。" } satisfies AppError),
    );
    request.send(file);
  });
}
