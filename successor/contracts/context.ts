import type { ConversationMemory, SkillBundle, TurnConfiguration } from './v1.ts';

export type ContextModuleRef = { id: string; version: string };
export type BundleContext = { skillId: string; revisionId: string; bundle: SkillBundle; textFiles: { path: string; text: string }[] };
export type ContextCompilation = { kind: 'ready'; instructions: string; bundles: BundleContext[]; memory?: ConversationMemory } | { kind: 'unsupported'; message: string };
export type ContextCompiler = ContextModuleRef & {
  protocolVersion: 1;
  compile(input: { provider: string; conversationId: string; configuration: TurnConfiguration }): ContextCompilation;
};
export type ContextModuleStatus = { state: 'missing' | 'disabled' } | { state: 'ready' | 'failed'; module: ContextModuleRef };
export type PreparedContext = { kind: 'empty' } | { kind: 'prepared'; module: ContextModuleRef; configurationDigest: string; content: Extract<ContextCompilation, { kind: 'ready' }> } | { kind: 'unavailable'; message: string };

export function parseContextModuleRef(value: unknown): ContextModuleRef {
  if (!value || typeof value !== 'object' || !('id' in value) || !('version' in value)
    || typeof value.id !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(value.id)
    || typeof value.version !== 'string' || !value.version.trim() || !value.version.isWellFormed() || Buffer.byteLength(value.version) > 128) throw new Error('Invalid context module identity');
  return { id: value.id, version: value.version };
}
