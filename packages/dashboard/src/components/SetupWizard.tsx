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

const MODEL_OPTIONS = [
  {
    tier: "fast" as const,
    name: "Fast",
    model: "Haiku",
    desc: "Quick responses, simple tasks. Lowest cost.",
  },
  {
    tier: "standard" as const,
    name: "Standard",
    model: "Sonnet",
    desc: "Balanced quality and speed. Best for most tasks.",
  },
  {
    tier: "powerful" as const,
    name: "Powerful",
    model: "Opus",
    desc: "Maximum reasoning. Complex planning and analysis.",
  },
];

export default function SetupWizard() {
  const navigate = useNavigate();
  const { send } = useFortSocket();
  const [visible, setVisible] = useState(false);
  const [step, setStep] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [data, setData] = useState({
    name: "Fort",
    goals: "",
    emoji: "🏰",
    personality: "",
    avatarDataUrl: "",
    modelTier: "standard" as "fast" | "standard" | "powerful",
  });
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchSetupStatus()
      .then((s) => {
        if (!s.complete) setVisible(true);
      })
      .catch(() => {});
  }, []);

  if (!visible) return null;

  const update = (fields: Partial<typeof data>) =>
    setData((prev) => ({ ...prev, ...fields }));

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
    setSubmitting(true);
    try {
      const result = await createAgent({
        name: data.name,
        goals: data.goals,
        emoji: data.emoji,
        personality: data.personality,
        avatarDataUrl: data.avatarDataUrl || null,
        modelTier: data.modelTier,
      });
      if (result.error) {
        alert("Error: " + result.error);
        setSubmitting(false);
        return;
      }
      setVisible(false);
      send("agents");
      send("status");

      // Navigate to chat — ChatPage handles auto-greeting
      setTimeout(() => {
        navigate(`/chat/${result.id}`);
      }, 1500);
    } catch {
      alert("Failed to create agent");
      setSubmitting(false);
    }
  };

  // 5 steps: Welcome (0), Name/Goals (1), Model (2), Emoji/Avatar (3), Summary (4)
  const totalSteps = 5;

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
            <h2>Choose a Model</h2>
            <p>Select the default AI model for this agent. You can override this per-message later.</p>
            <div className="model-selector">
              {MODEL_OPTIONS.map((m) => (
                <button
                  key={m.tier}
                  className={`model-option${data.modelTier === m.tier ? " selected" : ""}`}
                  onClick={() => update({ modelTier: m.tier })}
                >
                  <div className="model-option-name">{m.name}</div>
                  <div className="model-option-model">{m.model}</div>
                  <div className="model-option-desc">{m.desc}</div>
                </button>
              ))}
            </div>
            <div className="wizard-buttons">
              <button className="wizard-btn" onClick={() => setStep(1)}>Back</button>
              <button className="wizard-btn primary" onClick={() => setStep(3)}>Next</button>
            </div>
          </div>
        )}

        {step === 3 && (
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
              <button className="wizard-btn" onClick={() => setStep(2)}>Back</button>
              <button className="wizard-btn primary" onClick={() => setStep(4)}>Next</button>
            </div>
          </div>
        )}

        {step === 4 && (
          <div className="wizard-step">
            <h2>Summary</h2>
            <div className="wizard-summary">
              <div className="wizard-summary-emoji">{data.emoji}</div>
              <div className="wizard-summary-name">{data.name}</div>
              {data.goals && <div className="wizard-summary-goals">{data.goals}</div>}
              <div className="wizard-summary-model">
                Model: {MODEL_OPTIONS.find((m) => m.tier === data.modelTier)?.name} ({MODEL_OPTIONS.find((m) => m.tier === data.modelTier)?.model})
              </div>
            </div>
            <div className="wizard-buttons">
              <button className="wizard-btn" onClick={() => setStep(3)}>Back</button>
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
