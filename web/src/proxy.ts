import { NextResponse, type NextRequest } from "next/server";

const publicRoutes = new Set(["/", "/login", "/register"]);

export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  const hasSessionHint = request.cookies.get("im_session_hint")?.value === "1";

  if (!publicRoutes.has(pathname) && !hasSessionHint) {
    const login = new URL("/login", request.url);
    login.searchParams.set("return_to", `${pathname}${search}`);
    return NextResponse.redirect(login);
  }

  if ((pathname === "/login" || pathname === "/register") && hasSessionHint) {
    return NextResponse.redirect(new URL("/dashboard", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    "/dashboard/:path*",
    "/resumes/:path*",
    "/jobs/:path*",
    "/question-sets/:path*",
    "/interviews/:path*",
    "/tasks/:path*",
    "/settings/:path*",
    "/login",
    "/register",
  ],
};
