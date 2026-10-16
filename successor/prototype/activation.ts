export function requireCandidateBActivation(): never {
  throw new Error('Candidate B is not approved for execution. Fresh source/app pins, independent review and an exact execution decision are required.');
}

if (import.meta.main) requireCandidateBActivation();
