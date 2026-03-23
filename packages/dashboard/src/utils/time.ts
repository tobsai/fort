export function timeAgo(ts: string | Date): string {
  const now = Date.now();
  const then = typeof ts === "string" ? new Date(ts).getTime() : ts.getTime();
  const diff = Math.max(0, now - then);
  const secs = Math.floor(diff / 1000);

  if (secs < 60) return "just now";
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}
