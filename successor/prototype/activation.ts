import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';

export function requireCandidateBActivation(): void {
  const entry=process.argv[1];
  if(!entry || resolve(entry)!==fileURLToPath(new URL('./manual.ts',import.meta.url))) {
    throw new Error('Candidate B is approved only through the pinned manual packet.');
  }
}

if (import.meta.main) requireCandidateBActivation();
