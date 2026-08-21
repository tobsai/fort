import { issueNativeToken } from "@/lib/native-token";
import { nativeSessionIdentity } from "@/lib/session";

export async function POST(request: Request): Promise<Response> {
  const identity = await nativeSessionIdentity(request);
  if (!identity) {
    return Response.json({ error: "unauthorized" }, { status: 401 });
  }

  const token = await issueNativeToken(identity.email, process.env.AUTH_SECRET ?? "");
  return Response.json({ token });
}
