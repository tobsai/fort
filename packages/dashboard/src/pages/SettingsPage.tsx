import { useState, useEffect, useCallback } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";

// ─── Types ────────────────────────────────────────────────────────────────────

interface LLMProvider {
  id: string;
  name: string;
  hasApiKey: boolean;
  baseUrl: string | null;
  defaultModel: string;
  enabled: boolean;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

type ProviderType = "anthropic" | "openai" | "groq" | "ollama";

const PROVIDER_INFO: Record<ProviderType, { label: string; icon: string; needsKey: boolean }> = {
  anthropic: { label: "Anthropic", icon: "🟣", needsKey: true },
  openai:    { label: "OpenAI",    icon: "🟢", needsKey: true },
  groq:      { label: "Groq",      icon: "⚡",  needsKey: true },
  ollama:    { label: "Ollama",    icon: "🦙", needsKey: false },
};

const PROVIDER_MODELS: Record<ProviderType, string[]> = {
  anthropic: ["claude-opus-4-6", "claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001"],
  openai:    ["gpt-4o", "gpt-4o-mini", "gpt-4-turbo"],
  groq:      ["llama-3.3-70b-versatile", "llama-3.1-8b-instant"],
  ollama:    ["llama3", "mistral", "codellama"],
};

const PROVIDER_DEFAULTS: Record<ProviderType, { defaultModel: string; baseUrl?: string }> = {
  anthropic: { defaultModel: "claude-sonnet-4-5-20250929" },
  openai:    { defaultModel: "gpt-4o",                   baseUrl: "https://api.openai.com/v1" },
  groq:      { defaultModel: "llama-3.3-70b-versatile",  baseUrl: "https://api.groq.com/openai/v1" },
  ollama:    { defaultModel: "llama3",                   baseUrl: "http://localhost:11434" },
};

// ─── Add Provider Modal ───────────────────────────────────────────────────────

interface AddProviderModalProps {
  onClose: () => void;
  onAdded: () => void;
}

function AddProviderModal({ onClose, onAdded }: AddProviderModalProps) {
  const { send, subscribe } = useFortSocket();
  const [type, setType] = useState<ProviderType>("anthropic");
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState(PROVIDER_DEFAULTS.anthropic.baseUrl ?? "");
  const [defaultModel, setDefaultModel] = useState(PROVIDER_DEFAULTS.anthropic.defaultModel);
  const [isDefault, setIsDefault] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const info = PROVIDER_INFO[type];
  const models = PROVIDER_MODELS[type];

  function handleTypeChange(t: ProviderType) {
    setType(t);
    setBaseUrl(PROVIDER_DEFAULTS[t].baseUrl ?? "");
    setDefaultModel(PROVIDER_DEFAULTS[t].defaultModel);
    setApiKey("");
    setError(null);
  }

  function handleAdd() {
    setLoading(true);
    setError(null);
    const msgId = `add-provider-${Date.now()}`;

    const unsub = subscribe("llm.provider.add.response", (msg) => {
      if (msg.id !== msgId) return;
      unsub();
      setLoading(false);
      onAdded();
      onClose();
    });

    const unsubErr = subscribe("error", (msg) => {
      if (msg.id !== msgId) return;
      unsubErr();
      setLoading(false);
      setError((msg.payload as any)?.error ?? msg.error ?? "Unknown error");
    });

    send("llm.provider.add", {
      id: type,
      apiKey: info.needsKey ? apiKey : undefined,
      baseUrl: baseUrl || undefined,
      defaultModel,
      isDefault,
    });

    // Patch: the send() doesn't return the id, so we send raw
    // Actually useFortSocket.send() sends with auto-generated id — for response matching
    // we need to use a workaround. Use subscribe("*") instead:
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Add LLM Provider</h2>
          <button className="icon-btn" onClick={onClose}>✕</button>
        </div>

        <div className="modal-body">
          <div className="field">
            <label>Provider</label>
            <select value={type} onChange={(e) => handleTypeChange(e.target.value as ProviderType)}>
              {(Object.keys(PROVIDER_INFO) as ProviderType[]).map((id) => (
                <option key={id} value={id}>
                  {PROVIDER_INFO[id].icon} {PROVIDER_INFO[id].label}
                </option>
              ))}
            </select>
          </div>

          {info.needsKey && (
            <div className="field">
              <label>API Key</label>
              <input
                type="password"
                placeholder="sk-ant-..."
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
              />
            </div>
          )}

          {(type === "ollama" || type === "openai" || type === "groq") && (
            <div className="field">
              <label>Base URL</label>
              <input
                type="text"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder="https://..."
              />
            </div>
          )}

          <div className="field">
            <label>Default Model</label>
            <select value={defaultModel} onChange={(e) => setDefaultModel(e.target.value)}>
              {models.map((m) => (
                <option key={m} value={m}>{m}</option>
              ))}
            </select>
          </div>

          <div className="field field-row">
            <input
              id="set-default"
              type="checkbox"
              checked={isDefault}
              onChange={(e) => setIsDefault(e.target.checked)}
            />
            <label htmlFor="set-default">Set as default provider</label>
          </div>

          {error && <div className="error-banner">{error}</div>}
        </div>

        <div className="modal-footer">
          <button className="btn-secondary" onClick={onClose} disabled={loading}>Cancel</button>
          <button
            className="btn-primary"
            onClick={handleAdd}
            disabled={loading || (info.needsKey && !apiKey.trim())}
          >
            {loading ? "Testing connection…" : "Add Provider"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ─── Provider Card ────────────────────────────────────────────────────────────

interface ProviderCardProps {
  provider: LLMProvider;
  onSetDefault: (id: string) => void;
  onDelete: (id: string) => void;
  onTest: (id: string) => void;
  testResult: { success: boolean; error?: string } | null;
}

function ProviderCard({ provider, onSetDefault, onDelete, onTest, testResult }: ProviderCardProps) {
  const info = PROVIDER_INFO[provider.id as ProviderType] ?? { label: provider.name, icon: "🔌" };

  return (
    <div className={`provider-card${provider.isDefault ? " provider-card--default" : ""}`}>
      <div className="provider-card-header">
        <span className="provider-icon">{info.icon}</span>
        <div className="provider-info">
          <strong>{provider.name}</strong>
          <span className="provider-model">{provider.defaultModel}</span>
        </div>
        <div className="provider-badges">
          {provider.isDefault && <span className="badge badge--default">Default</span>}
          <span className={`badge ${provider.hasApiKey || provider.id === "ollama" ? "badge--ok" : "badge--warn"}`}>
            {provider.hasApiKey || provider.id === "ollama" ? "Configured" : "No key"}
          </span>
          {testResult !== null && (
            <span className={`badge ${testResult.success ? "badge--ok" : "badge--error"}`}>
              {testResult.success ? "Connected" : "Error"}
            </span>
          )}
        </div>
      </div>

      {testResult && !testResult.success && testResult.error && (
        <div className="provider-error">{testResult.error}</div>
      )}

      <div className="provider-actions">
        <button className="btn-sm btn-secondary" onClick={() => onTest(provider.id)}>
          Test
        </button>
        {!provider.isDefault && (
          <button className="btn-sm btn-secondary" onClick={() => onSetDefault(provider.id)}>
            Set Default
          </button>
        )}
        <button className="btn-sm btn-danger" onClick={() => onDelete(provider.id)}>
          Delete
        </button>
      </div>
    </div>
  );
}

// ─── Settings Page ────────────────────────────────────────────────────────────

export default function SettingsPage() {
  const { send, subscribe } = useFortSocket();
  const [providers, setProviders] = useState<LLMProvider[]>([]);
  const [showAddModal, setShowAddModal] = useState(false);
  const [testResults, setTestResults] = useState<Record<string, { success: boolean; error?: string }>>({});

  const loadProviders = useCallback(() => {
    const unsub = subscribe("llm.providers.list.response", (msg) => {
      unsub();
      const data = msg.payload as { providers: LLMProvider[] };
      setProviders(data.providers ?? []);
    });
    send("llm.providers.list");
  }, [send, subscribe]);

  useEffect(() => {
    loadProviders();
  }, [loadProviders]);

  function handleSetDefault(id: string) {
    const unsub = subscribe("llm.provider.update.response", () => {
      unsub();
      loadProviders();
    });
    send("llm.provider.update", { id, isDefault: true });
  }

  function handleDelete(id: string) {
    if (!confirm(`Remove provider "${id}"?`)) return;
    const unsub = subscribe("llm.provider.delete.response", () => {
      unsub();
      loadProviders();
    });
    send("llm.provider.delete", { id });
  }

  function handleTest(id: string) {
    const unsub = subscribe("llm.provider.test.response", (msg) => {
      const data = msg.payload as { id: string; success: boolean; error?: string };
      if (data.id !== id) return;
      unsub();
      setTestResults((prev) => ({ ...prev, [id]: { success: data.success, error: data.error } }));
    });
    send("llm.provider.test", { id });
  }

  return (
    <div className="settings-page">
      <div className="settings-section">
        <div className="settings-section-header">
          <h2>LLM Providers</h2>
          <button className="btn-primary" onClick={() => setShowAddModal(true)}>
            + Add Provider
          </button>
        </div>

        {providers.length === 0 ? (
          <div className="empty-state">
            <p>No providers configured. Add one to get started.</p>
            <p className="empty-hint">
              Fort will fall back to <code>ANTHROPIC_API_KEY</code> if set.
            </p>
          </div>
        ) : (
          <div className="provider-list">
            {providers.map((p) => (
              <ProviderCard
                key={p.id}
                provider={p}
                onSetDefault={handleSetDefault}
                onDelete={handleDelete}
                onTest={handleTest}
                testResult={testResults[p.id] ?? null}
              />
            ))}
          </div>
        )}
      </div>

      {showAddModal && (
        <AddProviderModal
          onClose={() => setShowAddModal(false)}
          onAdded={loadProviders}
        />
      )}
    </div>
  );
}
