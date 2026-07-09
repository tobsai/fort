// The sign-in page. Unauthenticated visitors land here (via middleware); an
// account that authenticates with Google but is NOT on FORT_ALLOWLIST is
// bounced back here with ?error=AccessDenied (Auth.js pages.error -> /signin).

import { signIn } from "@/auth";

export default function SignInPage({
  searchParams,
}: {
  searchParams: { callbackUrl?: string; error?: string };
}) {
  const callbackUrl = searchParams.callbackUrl ?? "/";
  const denied = searchParams.error === "AccessDenied";

  return (
    <div className="signin-wrap">
      <h1>Fort gateway</h1>
      <p className="subtitle">Sign in with your allowlisted Google account.</p>

      {denied ? (
        <div className="warn-banner" role="alert">
          <h3>Access denied</h3>
          <p>That Google account is not on this gateway&apos;s allowlist.</p>
        </div>
      ) : null}
      {searchParams.error && !denied ? (
        <p className="err">Sign-in error: {searchParams.error}</p>
      ) : null}

      <form
        action={async () => {
          "use server";
          await signIn("google", { redirectTo: callbackUrl });
        }}
      >
        <button type="submit" className="btn btn-primary">
          Continue with Google
        </button>
      </form>
    </div>
  );
}
