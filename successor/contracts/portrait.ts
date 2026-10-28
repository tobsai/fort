import type { AccountIdentity } from './v1.ts';

// Spec 065: what a portrait draw asks for and what comes back. Shared by the core and the executor.
// Sized so the base64 reply fits the Mac app's 8 MiB host channel.
export const PORTRAIT_MAX_BYTES = 5 * 1024 * 1024;
const PNG = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

export type PortraitSubject = { name: string; instructions: string; note?: string };
// Spec 067: the same run also draws a general image from the owner's own words.
export const IMAGE_PROMPT_BYTES = 2000;
export type PortraitRequest = { account: AccountIdentity; model: string } & ({ subject: PortraitSubject; prompt?: never } | { prompt: string; subject?: never });
export const validImagePrompt = (value: unknown): value is string => typeof value === 'string' && value.isWellFormed() && !value.includes('\0') && value.trim().length > 0 && Buffer.byteLength(value) <= IMAGE_PROMPT_BYTES;
export type PortraitResult =
  | { kind: 'drawn'; png: Buffer; width: number; height: number }
  | { kind: 'notStarted'; message: string }
  | { kind: 'failed'; reason: 'usageLimit' | 'refused' | 'noImage' | 'invalidImage'; message: string }
  | { kind: 'canceled'; message: string }
  | { kind: 'outcomeUnknown'; message: string };


// Accepts exactly one PNG within bounds; returns its dimensions. Codex picks the size itself, so near-square shapes are allowed and the apps crop to fit.
export function decodePortrait(base64: unknown): { png: Buffer; width: number; height: number } {
  if (typeof base64 !== 'string' || base64.length === 0 || base64.length > Math.ceil(PORTRAIT_MAX_BYTES / 3) * 4 || !/^[A-Za-z0-9+/]+={0,2}$/.test(base64) || base64.length % 4 !== 0) throw new Error('Invalid portrait encoding');
  const png = Buffer.from(base64, 'base64');
  if (png.length < 33 || png.length > PORTRAIT_MAX_BYTES || !png.subarray(0, 8).equals(PNG) || png.readUInt32BE(8) !== 13 || png.toString('latin1', 12, 16) !== 'IHDR') throw new Error('Portrait is not a PNG');
  const width = png.readUInt32BE(16), height = png.readUInt32BE(20);
  if ([width, height].some(side => side < 256 || side > 2048) || Math.max(width, height) > 2 * Math.min(width, height)) throw new Error('Portrait size is out of bounds');
  if (png.subarray(png.length - 12).toString('latin1', 4, 8) !== 'IEND') throw new Error('Portrait is truncated');
  return { png, width, height };
}

