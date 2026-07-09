// auth: the Auth.js (next-auth v5) configuration for the gateway web app.
//
// Google OIDC, JWT sessions, and an EMAIL ALLOWLIST enforced in the signIn
// callback: an authenticated Google account that is not on FORT_ALLOWLIST is
// rejected (the callback returns false -> Auth.js denies access). This is a
// personal, per-owner gateway (spec 028, D1) — not a multi-tenant login.
//
// The Google provider auto-reads AUTH_GOOGLE_ID / AUTH_GOOGLE_SECRET and the
// session cookie is signed with AUTH_SECRET (see .env.example).

import NextAuth from "next-auth";
import Google from "next-auth/providers/google";

import { isAllowed } from "@/lib/allowlist";

export const { handlers, auth, signIn, signOut } = NextAuth({
  providers: [Google],
  session: { strategy: "jwt" },
  pages: {
    signIn: "/signin",
    error: "/signin",
  },
  callbacks: {
    // Gate sign-in on the allowlist. Returning false surfaces as AccessDenied.
    signIn({ user }) {
      return isAllowed(user.email, process.env.FORT_ALLOWLIST);
    },
  },
});
