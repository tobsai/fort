import { SignJWT, jwtVerify } from "jose";

const audience = "fort-native";
export const nativeSessionLifetimeSeconds = 30 * 24 * 60 * 60;

function key(secret: string): Uint8Array {
  if (!secret) throw new Error("AUTH_SECRET is not set");
  return new TextEncoder().encode(secret);
}

export async function issueNativeToken(
  email: string,
  secret: string,
  nowSeconds = Math.floor(Date.now() / 1_000),
): Promise<string> {
  return new SignJWT({ email })
    .setProtectedHeader({ alg: "HS256", typ: "JWT" })
    .setAudience(audience)
    .setIssuedAt(nowSeconds)
    .setJti(crypto.randomUUID())
    .setExpirationTime(nowSeconds + nativeSessionLifetimeSeconds)
    .sign(key(secret));
}

export async function verifyNativeToken(
  token: string,
  secret: string,
  nowSeconds = Math.floor(Date.now() / 1_000),
): Promise<{ email: string }> {
  let payload;
  try {
    ({ payload } = await jwtVerify(token, key(secret), {
      algorithms: ["HS256"],
      audience,
      currentDate: new Date(nowSeconds * 1_000),
    }));
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ERR_JWT_EXPIRED") {
      throw new Error("native token expired");
    }
    throw error;
  }
  if (typeof payload.email !== "string" || !payload.email) {
    throw new Error("native token has no email");
  }
  return { email: payload.email };
}
