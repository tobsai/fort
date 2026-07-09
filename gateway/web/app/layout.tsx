import type { Metadata } from "next";
import Link from "next/link";

import "./globals.css";
import { auth, signOut } from "@/auth";

export const metadata: Metadata = {
  title: "Fort gateway",
  description: "Drive your Forts from anywhere — a self-hosted, E2E-encrypted gateway.",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await auth();
  return (
    <html lang="en">
      <body>
        <header className="topbar">
          <Link href="/" className="brand">
            fort<span className="brand-dim">·gateway</span>
          </Link>
          {session?.user ? (
            <nav className="nav">
              <Link href="/">Machines</Link>
              <Link href="/add">Add machine</Link>
              <form
                action={async () => {
                  "use server";
                  await signOut({ redirectTo: "/signin" });
                }}
              >
                <button type="submit" className="link-btn">
                  Sign out{session.user.email ? ` (${session.user.email})` : ""}
                </button>
              </form>
            </nav>
          ) : null}
        </header>
        <main className="container">{children}</main>
      </body>
    </html>
  );
}
