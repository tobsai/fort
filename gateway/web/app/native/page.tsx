import { redirect } from "next/navigation";

import { auth } from "@/auth";
import { issueNativeToken } from "@/lib/native-token";

export default async function NativeSignInComplete(): Promise<never> {
  const session = await auth();
  const email = session?.user?.email;
  if (!email) redirect("/signin?callbackUrl=/native");

  const token = await issueNativeToken(email, process.env.AUTH_SECRET ?? "");
  const callback = new URL("fort://auth");
  callback.searchParams.set("token", token);
  redirect(callback.toString());
}
