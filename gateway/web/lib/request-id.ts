export const FORT_REQUEST_ID_HEADER = "X-Fort-Request-ID";

const canonicalUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export function isCanonicalRequestID(value: string | null | undefined): value is string {
  return typeof value === "string" && canonicalUUID.test(value);
}

export function newRequestID(): string {
  return crypto.randomUUID();
}

export function requestIDFrom(request: Request): string {
  const supplied = request.headers.get(FORT_REQUEST_ID_HEADER);
  return isCanonicalRequestID(supplied) ? supplied : newRequestID();
}

export function correlatedJSON(body: unknown, status: number, requestID: string): Response {
  return Response.json(body, {
    status,
    headers: { [FORT_REQUEST_ID_HEADER]: requestID },
  });
}
