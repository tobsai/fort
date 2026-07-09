// middleware: gate every app page and internal API route behind a valid
// session. Auth.js's `auth` wrapper populates req.auth; unauthenticated
// requests to a protected path are redirected to the sign-in page.
//
// The matcher deliberately EXCLUDES the Auth.js routes (/api/auth/*), the
// sign-in page, and Next's static assets so the login flow itself is reachable.

import { auth } from "@/auth";

export default auth((req) => {
  if (!req.auth) {
    const url = new URL("/signin", req.nextUrl.origin);
    url.searchParams.set("callbackUrl", req.nextUrl.pathname + req.nextUrl.search);
    return Response.redirect(url);
  }
  // Authenticated: let the request through unchanged.
  return undefined;
});

export const config = {
  // Protect everything except: Auth.js endpoints, the sign-in page, and static
  // assets / the favicon.
  matcher: ["/((?!api/auth|signin|_next/static|_next/image|favicon.ico).*)"],
};
