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
  provider?: "anthropic" | "openai" | "grok" | "groq" | "google" | "ollama" | "openrouter";
  isDefault?: boolean;
}): Promise<{ id: string; name: string; emoji: string; error?: string }> {
  const res = await fetch(`${BASE}/api/agents/create`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  return res.json();
}

export interface ConfiguredProvidersResponse {
  providers: Array<{
    id: string;
    name: string;
    usable: boolean;
    authMethod: string | null;
    models: { fast: string; standard: string; powerful: string };
    hint?: string;
  }>;
  defaultProviderId: string | null;
  agentDefaultExists: boolean;
}

export async function fetchConfiguredProviders(): Promise<ConfiguredProvidersResponse> {
  const res = await fetch(`${BASE}/api/providers/configured`);
  return res.json();
}

export async function connectSubscription(
  providerId: "anthropic" | "openai",
): Promise<{ ok: boolean; pollUrl?: string; error?: string; hint?: string }> {
  const res = await fetch(`${BASE}/api/providers/connect-subscription`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ providerId }),
  });
  return res.json();
}

export async function pollSubscriptionStatus(
  providerId: "anthropic" | "openai",
): Promise<{ ready: boolean; authMethod: string | null; error?: string }> {
  const res = await fetch(`${BASE}/api/providers/subscription-status?providerId=${providerId}`);
  return res.json();
}
