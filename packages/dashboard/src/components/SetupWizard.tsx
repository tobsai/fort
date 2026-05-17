import { useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import {
  fetchSetupStatus,
  createAgent,
  fetchConfiguredProviders,
  connectSubscription,
  pollSubscriptionStatus,
} from "../utils/api";
import { useFortSocket } from "../contexts/FortSocketContext";

const EMOJI_OPTIONS = [
  "🏰", "🤖", "🦉", "🧠",
  "⚡", "🛡️", "🔮", "🌟",
  "🐙", "🎯", "🔥", "🌊",
  "🗡️", "🎭", "📡", "🧬",
];

const PROVIDER_ICONS: Record<string, string> = {
  anthropic:  "🟣",
  openai:     "🟢",
  grok:       "⚪",
  groq:       "⚡",
  google:     "🔵",
  ollama:     "🦙",
  openrouter: "🛣️",
};

type ProviderId = "anthropic" | "openai" | "grok" | "groq" | "google" | "ollama" | "openrouter";
type ModelTier = "fast" | "standard" | "powerful";

interface AvailableProvider {
  id: ProviderId;
  name: string;
  usable: boolean;
  authMethod: string | null;
  models: { fast: string; standard: string; powerful: string };
  hint?: string;
}

const TIER_LABELS: Record<ModelTier, { name: string; desc: string }> = {
  fast:     { name: "Fast",     desc: "Quick responses, simple tasks. Lowest cost." },
  standard: { name: "Standard", desc: "Balanced quality and speed. Best for most tasks." },
  powerful: { name: "Powerful", desc: "Maximum reasoning. Complex planning and analysis." },
};

const KEY_PROMPTS: Record<ProviderId, { envVar: string; placeholder: string; signupUrl: string } | null> = {
  anthropic:  { envVar: "ANTHROPIC_API_KEY",   placeholder: "sk-ant-...",  signupUrl: "https://console.anthropic.com/settings/keys" },
  openai:     { envVar: "OPENAI_API_KEY",      placeholder: "sk-...",      signupUrl: "https://platform.openai.com/api-keys" },
  grok:       { envVar: "XAI_API_KEY",         placeholder: "xai-...",     signupUrl: "https://console.x.ai" },
  groq:       { envVar: "GROQ_API_KEY",        placeholder: "gsk_...",     signupUrl: "https://console.groq.com/keys" },
  google:     { envVar: "GEMINI_API_KEY",      placeholder: "AIza...",     signupUrl: "https://aistudio.google.com/apikey" },
  openrouter: { envVar: "OPENROUTER_API_KEY",  placeholder: "sk-or-...",   signupUrl: "https://openrouter.ai/keys" },
  ollama:     null, // no key — runs locally
};

interface InlineSetupProps {
  providerId: ProviderId;
  onDone: () => void;
  onCancel: () => void;
}

const SUBSCRIPTION_PROVIDERS = new Set<ProviderId>(["anthropic", "openai"]);
const SUBSCRIPTION_LABEL: Record<string, string> = {
  anthropic: "Claude subscription",
  openai: "ChatGPT/Codex subscription",
};

type SubStatus = "idle" | "waiting" | "done" | "timeout" | "error";

function InlineSetup({ providerId, onDone, onCancel }: InlineSetupProps) {
  const [key, setKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [mode, setMode] = useState<"choose" | "subscription" | "apikey">(
    SUBSCRIPTION_PROVIDERS.has(providerId) ? "choose" : "apikey",
  );
  const [subStatus, setSubStatus] = useState<SubStatus>("idle");
  const [subMessage, setSubMessage] = useState<string | null>(null);
  const pollHandle = useRef<{ stopped: boolean }>({ stopped: false });
  const spec = KEY_PROMPTS[providerId];

  useEffect(() => () => { pollHandle.current.stopped = true; }, []);

  async function startSubscription() {
    if (!SUBSCRIPTION_PROVIDERS.has(providerId)) return;
    setMode("subscription");
    setSubStatus("waiting");
    setSubMessage("Opening Terminal — finish the login there, this page will detect it.");
    pollHandle.current.stopped = false;

    const start = await connectSubscription(providerId as "anthropic" | "openai");
    if (!start.ok) {
      setSubStatus("error");
      setSubMessage(start.hint ?? start.error ?? "Could not start subscription login.");
      return;
    }

    const deadline = Date.now() + 120_000;
    let fetchFailures = 0;
    while (!pollHandle.current.stopped) {
      if (Date.now() > deadline) {
        setSubStatus("timeout");
        setSubMessage("Login took too long. Try again or paste an API key instead.");
        return;
      }
      try {
        const s = await pollSubscriptionStatus(providerId as "anthropic" | "openai");
        if (s.ready) {
          setSubStatus("done");
          setSubMessage(`Connected via ${s.authMethod ?? "subscription"}.`);
          // Brief pause so users see the success state, then close.
          setTimeout(() => { if (!pollHandle.current.stopped) onDone(); }, 800);
          return;
        }
        fetchFailures = 0;
      } catch {
        fetchFailures += 1;
        if (fetchFailures >= 3) {
          setSubStatus("error");
          setSubMessage("Lost connection to Fort. Retry?");
          return;
        }
      }
      await new Promise((r) => setTimeout(r, 2000));
    }
  }

  function cancelSubscription() {
    pollHandle.current.stopped = true;
    setSubStatus("idle");
    setMode("choose");
  }

  async function submitApiKey() {
    setSubmitting(true);
    setError(null);
    try {
      const body = spec ? { id: providerId, key: key.trim() } : { id: providerId };
      const res = await fetch("/api/providers/setup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await res.json();
      if (!res.ok || data.error) {
        setError(data.error ?? `HTTP ${res.status}`);
        setSubmitting(false);
        return;
      }
      if (!data.usable) {
        setError(data.hint ?? "Provider still not usable.");
        setSubmitting(false);
        return;
      }
      onDone();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setSubmitting(false);
    }
  }

  // Subscription-capable providers default to a choice between subscription and API key.
  if (mode === "choose" && SUBSCRIPTION_PROVIDERS.has(providerId)) {
    return (
      <div className="inline-setup">
        <p className="inline-setup-help">
          Connect your {SUBSCRIPTION_LABEL[providerId]} — no API key needed.
        </p>
        <div className="inline-setup-actions">
          <button className="btn-sm btn-secondary" onClick={onCancel}>Cancel</button>
          <button className="btn-sm" onClick={() => setMode("apikey")}>Use API key instead</button>
          <button className="btn-sm btn-primary" onClick={startSubscription}>
            Connect {SUBSCRIPTION_LABEL[providerId]}
          </button>
        </div>
      </div>
    );
  }

  if (mode === "subscription") {
    return (
      <div className="inline-setup">
        <p className="inline-setup-help">{subMessage}</p>
        {subStatus === "waiting" && (
          <div className="inline-setup-actions">
            <button className="btn-sm btn-secondary" onClick={cancelSubscription}>Cancel</button>
          </div>
        )}
        {(subStatus === "timeout" || subStatus === "error") && (
          <div className="inline-setup-actions">
            <button className="btn-sm" onClick={() => setMode("apikey")}>Use API key instead</button>
            <button className="btn-sm btn-primary" onClick={startSubscription}>Retry</button>
          </div>
        )}
        {subStatus === "done" && (
          <div className="inline-setup-success">✓ {subMessage}</div>
        )}
      </div>
    );
  }

  return (
    <div className="inline-setup">
      {spec ? (
        <>
          <p className="inline-setup-help">
            Get a key at <a href={spec.signupUrl} target="_blank" rel="noreferrer">{spec.signupUrl}</a>
          </p>
          <input
            type="password"
            placeholder={spec.placeholder}
            value={key}
            onChange={(e) => setKey(e.target.value)}
            autoFocus
          />
          <p className="inline-setup-note">
            Stored as <code>{spec.envVar}</code> in <code>~/.fort/.env</code>.
          </p>
        </>
      ) : (
        <p className="inline-setup-help">
          Ollama runs locally. Make sure it's running:{" "}
          <code>ollama serve</code>
        </p>
      )}
      {error && <div className="inline-setup-error">{error}</div>}
      <div className="inline-setup-actions">
        <button className="btn-sm btn-secondary" onClick={onCancel} disabled={submitting}>Cancel</button>
        {SUBSCRIPTION_PROVIDERS.has(providerId) && (
          <button className="btn-sm" onClick={() => setMode("choose")} disabled={submitting}>
            Use subscription instead
          </button>
        )}
        <button
          className="btn-sm btn-primary"
          onClick={submitApiKey}
          disabled={submitting || (spec !== null && !key.trim())}
        >
          {submitting ? "Saving..." : (spec ? "Save & verify" : "Retry check")}
        </button>
      </div>
    </div>
  );
}

export default function SetupWizard() {
  const navigate = useNavigate();
  const { send, subscribe } = useFortSocket();
  const [visible, setVisible] = useState(false);
  const [step, setStep] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [providers, setProviders] = useState<AvailableProvider[]>([]);
  const [expandedSetup, setExpandedSetup] = useState<ProviderId | null>(null);
  const [data, setData] = useState({
    name: "Fort",
    goals: "",
    emoji: "🏰",
    personality: "",
    avatarDataUrl: "",
    provider: null as ProviderId | null,
    modelTier: "standard" as ModelTier,
  });
  const fileRef = useRef<HTMLInputElement>(null);

  // Show wizard only on first run. If providers are already configured (from
  // the CLI), jump straight to step 3 so the user isn't re-prompted.
  useEffect(() => {
    fetchSetupStatus()
      .then(async (s) => {
        if (s.complete) return;
        setVisible(true);
        try {
          const cfg = await fetchConfiguredProviders();
          if (cfg.providers?.length) setProviders(cfg.providers as AvailableProvider[]);
          if (cfg.defaultProviderId) {
            setData((prev) => ({ ...prev, provider: cfg.defaultProviderId as ProviderId }));
          }
          const hasUsable = cfg.providers?.some((p) => p.usable);
          if (hasUsable && !cfg.agentDefaultExists) {
            setStep(3);
          }
        } catch { /* fall through to step 0 */ }
      })
      .catch(() => {});
  }, []);

  // Refresh provider list when the wizard becomes visible (in case CLI changes things mid-flow).
  useEffect(() => {
    if (!visible) return;
    const unsub = subscribe("providers.available.response", (msg) => {
      const payload = msg.payload as { providers?: AvailableProvider[] };
      if (payload?.providers) setProviders(payload.providers);
    });
    send("providers.available");
    return () => unsub();
  }, [visible, send, subscribe]);

  const refreshProviders = async () => {
    try {
      const cfg = await fetchConfiguredProviders();
      if (cfg.providers?.length) setProviders(cfg.providers as AvailableProvider[]);
    } catch { /* fall back to ws refresh */ }
    send("providers.available");
  };

  // Default the wizard to the first usable provider once data lands.
  useEffect(() => {
    if (data.provider) return;
    const firstUsable = providers.find((p) => p.usable);
    if (firstUsable) setData((prev) => ({ ...prev, provider: firstUsable.id }));
  }, [providers, data.provider]);

  if (!visible) return null;

  const update = (fields: Partial<typeof data>) => setData((prev) => ({ ...prev, ...fields }));

  const handleAvatarUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > 5 * 1024 * 1024) {
      alert("Image must be under 5MB");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => update({ avatarDataUrl: reader.result as string });
    reader.readAsDataURL(file);
  };

  const handleSubmit = async () => {
    if (!data.provider) {
      alert("Pick a provider first.");
      return;
    }
    setSubmitting(true);
    try {
      const result = await createAgent({
        name: data.name,
        goals: data.goals,
        emoji: data.emoji,
        personality: data.personality,
        avatarDataUrl: data.avatarDataUrl || null,
        modelTier: data.modelTier,
        provider: data.provider,
      });
      if (result.error) {
        alert("Error: " + result.error);
        setSubmitting(false);
        return;
      }
      setVisible(false);
      send("agents");
      send("status");
      setTimeout(() => { navigate(`/chat/${result.id}`); }, 1500);
    } catch {
      alert("Failed to create agent");
      setSubmitting(false);
    }
  };

  // 6 steps: Welcome (0), Name/Goals (1), Provider (2), Model (3), Emoji/Avatar (4), Summary (5)
  const totalSteps = 6;
  const selectedProvider = providers.find((p) => p.id === data.provider);

  return (
    <div className="wizard-overlay active">
      <div className="wizard-card">
        <div className="wizard-progress">
          {Array.from({ length: totalSteps }, (_, i) => (
            <span
              key={i}
              className={`wizard-dot${i === step ? " active" : ""}${i < step ? " done" : ""}`}
            />
          ))}
        </div>

        {step === 0 && (
          <div className="wizard-step">
            <div className="wizard-emoji">🏰</div>
            <h2>Welcome to Fort</h2>
            <p>Let's create your first AI agent. This will be your default assistant.</p>
            <button className="wizard-btn primary" onClick={() => setStep(1)}>
              Get Started
            </button>
          </div>
        )}

        {step === 1 && (
          <div className="wizard-step">
            <h2>Name & Goals</h2>
            <label>
              Agent Name
              <input
                value={data.name}
                onChange={(e) => update({ name: e.target.value })}
                placeholder="Fort"
              />
            </label>
            <label>
              What should this agent help you with?
              <textarea
                value={data.goals}
                onChange={(e) => update({ goals: e.target.value })}
                rows={3}
                placeholder="e.g. Help me manage my tasks, write code, research topics..."
              />
            </label>
            <label>
              Personality
              <textarea
                value={data.personality}
                onChange={(e) => update({ personality: e.target.value })}
                rows={2}
                placeholder="e.g. Concise and direct, with dry humor..."
              />
            </label>
            <div className="wizard-buttons">
              <button className="wizard-btn" onClick={() => setStep(0)}>Back</button>
              <button className="wizard-btn primary" onClick={() => setStep(2)}>Next</button>
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="wizard-step">
            <h2>Choose Providers</h2>
            <p>Connect one or more LLM providers — Claude and ChatGPT subscriptions both work. You can pick which one this agent uses on the next step, and future agents can use any of them.</p>
            {providers.length === 0 ? (
              <div className="wizard-loading">Detecting available providers...</div>
            ) : (
              <div className="provider-selector">
                {providers.map((p) => (
                  <div key={p.id} className="provider-option-wrap">
                    <button
                      className={`provider-option${data.provider === p.id ? " selected" : ""}${p.usable ? "" : " unconfigured"}`}
                      onClick={() => {
                        if (p.usable) {
                          update({ provider: p.id });
                          setExpandedSetup(null);
                        } else {
                          setExpandedSetup(expandedSetup === p.id ? null : p.id);
                        }
                      }}
                    >
                      <div className="provider-option-icon">{PROVIDER_ICONS[p.id] ?? "🔌"}</div>
                      <div className="provider-option-body">
                        <div className="provider-option-name">{p.name}</div>
                        <div className="provider-option-meta">
                          {p.usable ? (
                            <span className="badge badge--ok">Configured</span>
                          ) : (
                            <span className="badge badge--warn">
                              {expandedSetup === p.id ? "Set up below ↓" : "Set up →"}
                            </span>
                          )}
                        </div>
                        {!p.usable && p.hint && expandedSetup !== p.id && (
                          <div className="provider-option-hint">{p.hint}</div>
                        )}
                      </div>
                    </button>
                    {expandedSetup === p.id && (
                      <InlineSetup
                        providerId={p.id}
                        onDone={() => { setExpandedSetup(null); refreshProviders(); }}
                        onCancel={() => setExpandedSetup(null)}
                      />
                    )}
                  </div>
                ))}
              </div>
            )}
            <div className="wizard-buttons">
              <button className="wizard-btn" onClick={() => setStep(1)}>Back</button>
              <button
                className="wizard-btn primary"
                onClick={() => setStep(3)}
                disabled={!data.provider}
              >
                Next
              </button>
            </div>
          </div>
        )}

        {step === 3 && (
          <div className="wizard-step">
            <h2>Provider & Model</h2>
            <p>Pick which provider this agent should use, and its default tier. You can override per-message later, and each future agent (Triager, doers) can have its own.</p>
            {providers.filter((p) => p.usable).length > 1 && (
              <div className="provider-selector compact">
                {providers.filter((p) => p.usable).map((p) => (
                  <button
                    key={p.id}
                    className={`provider-option${data.provider === p.id ? " selected" : ""}`}
                    onClick={() => update({ provider: p.id })}
                  >
                    <div className="provider-option-icon">{PROVIDER_ICONS[p.id] ?? "🔌"}</div>
                    <div className="provider-option-body">
                      <div className="provider-option-name">{p.name}</div>
                      <div className="provider-option-meta">
                        <span className="badge badge--ok">{p.authMethod ?? "configured"}</span>
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            )}
            {selectedProvider ? (
              <div className="model-selector">
                {(["fast", "standard", "powerful"] as ModelTier[]).map((tier) => (
                  <button
                    key={tier}
                    className={`model-option${data.modelTier === tier ? " selected" : ""}`}
                    onClick={() => update({ modelTier: tier })}
                  >
                    <div className="model-option-name">{TIER_LABELS[tier].name}</div>
                    <div className="model-option-model">{selectedProvider.models[tier]}</div>
                    <div className="model-option-desc">{TIER_LABELS[tier].desc}</div>
                  </button>
                ))}
              </div>
            ) : (
              <div className="wizard-loading">No provider selected.</div>
            )}
            <div className="wizard-buttons">
              <button className="wizard-btn" onClick={() => setStep(2)}>Back</button>
              <button className="wizard-btn primary" onClick={() => setStep(4)}>Next</button>
            </div>
          </div>
        )}

        {step === 4 && (
          <div className="wizard-step">
            <h2>Choose an Emoji</h2>
            <div className="emoji-grid">
              {EMOJI_OPTIONS.map((e) => (
                <button
                  key={e}
                  className={`emoji-option${data.emoji === e ? " selected" : ""}`}
                  onClick={() => update({ emoji: e })}
                >
                  {e}
                </button>
              ))}
            </div>
            <div className="avatar-section">
              <div className="avatar-preview">
                {data.avatarDataUrl ? (
                  <img src={data.avatarDataUrl} alt="avatar" />
                ) : (
                  <img src="/api/default-avatar" alt="default avatar" />
                )}
              </div>
              <input
                ref={fileRef}
                type="file"
                accept="image/png,image/jpeg,image/webp"
                style={{ display: "none" }}
                onChange={handleAvatarUpload}
              />
              <button className="wizard-btn" onClick={() => fileRef.current?.click()}>
                Upload Avatar
              </button>
              {data.avatarDataUrl && (
                <button className="wizard-btn" onClick={() => update({ avatarDataUrl: "" })}>
                  Use Default
                </button>
              )}
            </div>
            <div className="wizard-buttons">
              <button className="wizard-btn" onClick={() => setStep(3)}>Back</button>
              <button className="wizard-btn primary" onClick={() => setStep(5)}>Next</button>
            </div>
          </div>
        )}

        {step === 5 && (
          <div className="wizard-step">
            <h2>Summary</h2>
            <div className="wizard-summary">
              <div className="wizard-summary-emoji">{data.emoji}</div>
              <div className="wizard-summary-name">{data.name}</div>
              {data.goals && <div className="wizard-summary-goals">{data.goals}</div>}
              <div className="wizard-summary-model">
                Provider: {selectedProvider?.name ?? "—"}
              </div>
              <div className="wizard-summary-model">
                Model: {TIER_LABELS[data.modelTier].name}
                {selectedProvider ? ` (${selectedProvider.models[data.modelTier]})` : ""}
              </div>
            </div>
            <div className="wizard-buttons">
              <button className="wizard-btn" onClick={() => setStep(4)}>Back</button>
              <button
                className="wizard-btn primary"
                onClick={handleSubmit}
                disabled={submitting}
              >
                {submitting ? "Creating agent..." : "Launch Fort"}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
