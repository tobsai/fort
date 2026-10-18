export type RuntimeAttempt = Readonly<{
  runId: string;
  attemptId: string;
  generation: number;
  bindingRevisionId: string;
  runtimeVersion: string;
  selectedModel: string;
}>;
