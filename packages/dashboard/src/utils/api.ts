const BASE = "";

export async function fetchSetupStatus(): Promise<{ complete: boolean }> {
  const res = await fetch(`${BASE}/api/setup-status`);
  return res.json();
}

export async function fetchChatHistory(): Promise<
  Array<{
    id: string;
    shortId: string;
    title: string;
    description: string;
    result?: string;
    status: string;
    source: string;
    assignedAgent?: string;
    createdAt: string;
  }>
> {
  const res = await fetch(`${BASE}/api/chat-history`);
  return res.json();
}

export async function fetchLLMStatus(): Promise<{
  configured: boolean;
  authMethod: string | null;
  valid: boolean;
  error?: string;
}> {
  const res = await fetch(`${BASE}/api/llm-status`);
  return res.json();
}

export async function createAgent(data: {
  name: string;
  goals: string;
  emoji: string;
  personality: string;
  avatarDataUrl?: string | null;
  modelTier?: "fast" | "standard" | "powerful";
}): Promise<{ id: string; name: string; emoji: string; error?: string }> {
  const res = await fetch(`${BASE}/api/agents/create`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  return res.json();
}
