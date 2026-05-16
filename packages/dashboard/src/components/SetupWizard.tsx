import { useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { fetchSetupStatus, createAgent } from "../utils/api";
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

export default function SetupWizard() {
  const navigate = useNavigate();
  const { send, subscribe } = useFortSocket();
  const [visible, setVisible] = useState(false);
  const [step, setStep] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [providers, setProviders] = useState<AvailableProvider[]>([]);
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

  // Show wizard only on first run.
  useEffect(() => {
    fetchSetupStatus()
      .then((s) => { if (!s.complete) setVisible(true); })
      .catch(() => {});
  }, []);

  // Pull provider list once the wizard becomes visible.
  useEffect(() => {
    if (!visible) return;
    const unsub = subscribe("providers.available.response", (msg) => {
      const payload = msg.payload as { providers?: AvailableProvider[] };
      if (payload?.providers) setProviders(payload.providers);
    });
    send("providers.available");
    return () => unsub();
  }, [visible, send, subscribe]);

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
            <h2>Choose a Provider</h2>
            <p>This will be your default LLM provider. You can change it later in Settings.</p>
            {providers.length === 0 ? (
              <div className="wizard-loading">Detecting available providers...</div>
            ) : (
              <div className="provider-selector">
                {providers.map((p) => (
                  <button
                    key={p.id}
                    className={`provider-option${data.provider === p.id ? " selected" : ""}${p.usable ? "" : " disabled"}`}
                    onClick={() => p.usable && update({ provider: p.id })}
                    disabled={!p.usable}
                    title={p.hint}
                  >
                    <div className="provider-option-icon">{PROVIDER_ICONS[p.id] ?? "🔌"}</div>
                    <div className="provider-option-body">
                      <div className="provider-option-name">{p.name}</div>
                      <div className="provider-option-meta">
                        {p.usable ? (
                          <span className="badge badge--ok">Configured</span>
                        ) : (
                          <span className="badge badge--warn">Not configured</span>
                        )}
                      </div>
                      {!p.usable && p.hint && (
                        <div className="provider-option-hint">{p.hint}</div>
                      )}
                    </div>
                  </button>
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
            <h2>Choose a Model</h2>
            <p>Select the default tier for this agent. You can override per-message later.</p>
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
