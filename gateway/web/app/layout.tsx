import type { Metadata } from "next";
import Link from "next/link";

import "./globals.css";
import { auth, signOut } from "@/auth";

export const metadata: Metadata = {
  title: "Fort — Chat with your Agents",
  description: "Durable conversations, Groups, Routines, and Handoffs across exact agent frameworks and computers.",
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
            <span className="fort-mark" aria-hidden="true" data-motion="ambient">
              <span className="fort-mark-core" />
              <span className="fort-mark-orbit fort-mark-orbit-a" />
              <span className="fort-mark-orbit fort-mark-orbit-b" />
            </span>
            <span>FORT</span> <span className="brand-context">AGENT CHAT</span>
          </Link>
          {session?.user ? (
            <nav className="nav">
              <Link href="/">Agents</Link>
              <Link href="/groups">Groups</Link>
              <Link href="/handoffs">Handoffs</Link>
              <Link href="/legacy">Legacy</Link>
              <form
                action={async () => {
                  "use server";
                  await signOut({ redirectTo: "/signin" });
                }}
              >
                <button type="submit" className="link-btn">
                  Sign out{session.user.email ? ` · ${session.user.email}` : ""}
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
