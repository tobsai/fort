import { describe, it, expect, vi } from 'vitest';
import { classifyAsTask, decomposeTask, formatPlan } from '../agents/task-planner.js';

function makeMockLLM(responses: string[]): any {
  let i = 0;
  return {
    isConfigured: true,
    complete: vi.fn(async () => ({
      content: responses[Math.min(i++, responses.length - 1)],
      model: 'mock', inputTokens: 0, outputTokens: 0, totalTokens: 0, costUsd: 0,
      stopReason: 'end_turn', durationMs: 1,
    })),
  };
}

describe('task-planner', () => {
  describe('classifyAsTask', () => {
    it('parses a positive classification', async () => {
      const llm = makeMockLLM(['{"isTask": true, "confidence": 0.92, "summary": "Plan a Lisbon trip"}']);
      const out = await classifyAsTask(llm, 'Plan a 3-day trip to Lisbon');
      expect(out.isTask).toBe(true);
      expect(out.confidence).toBe(0.92);
      expect(out.summary).toBe('Plan a Lisbon trip');
    });

    it('parses a negative classification', async () => {
      const llm = makeMockLLM(['{"isTask": false, "confidence": 0.95, "summary": "single-turn question"}']);
      const out = await classifyAsTask(llm, "What's the capital of France?");
      expect(out.isTask).toBe(false);
    });

    it('strips ```json fences from the response', async () => {
      const llm = makeMockLLM(['```json\n{"isTask": true, "confidence": 0.7, "summary": "ok"}\n```']);
      const out = await classifyAsTask(llm, 'help me move apartments');
      expect(out.isTask).toBe(true);
      expect(out.confidence).toBe(0.7);
    });

    it('returns isTask:false when output is unparseable', async () => {
      const llm = makeMockLLM(['not json at all']);
      const out = await classifyAsTask(llm, 'whatever');
      expect(out.isTask).toBe(false);
      expect(out.confidence).toBe(0);
    });

    it('clamps confidence to [0, 1]', async () => {
      const llm = makeMockLLM(['{"isTask": true, "confidence": 1.7, "summary": "x"}']);
      const out = await classifyAsTask(llm, 'x');
      expect(out.confidence).toBe(1);
    });

    it('returns isTask:false when LLM is not configured', async () => {
      const out = await classifyAsTask({ isConfigured: false } as any, 'anything');
      expect(out.isTask).toBe(false);
    });
  });

  describe('decomposeTask', () => {
    it('parses a well-formed subtask list', async () => {
      const llm = makeMockLLM([
        '{"subtasks": [' +
          '{"title": "Find flights", "description": "Search and compare 3 round-trip options"},' +
          '{"title": "Book hotel",  "description": "Pick a 4-star in Alfama","expectedOutcome": "Confirmation email"}' +
        ']}'
      ]);
      const out = await decomposeTask(llm, 'Plan Lisbon trip', { summary: 'lisbon trip' });
      expect(out.subtasks.length).toBe(2);
      expect(out.subtasks[0].title).toBe('Find flights');
      expect(out.subtasks[1].expectedOutcome).toBe('Confirmation email');
    });

    it('caps subtasks at MAX_SUBTASKS (6)', async () => {
      const many = Array.from({ length: 12 }, (_, i) => `{"title":"t${i}","description":"d${i}"}`).join(',');
      const llm = makeMockLLM([`{"subtasks": [${many}]}`]);
      const out = await decomposeTask(llm, 'big task', { summary: 'big' });
      expect(out.subtasks.length).toBe(6);
    });

    it('skips entries missing title or description', async () => {
      const llm = makeMockLLM([
        '{"subtasks": [' +
          '{"title": "ok", "description": "fine"},' +
          '{"title": "no desc"},' +
          '{"description": "no title"}' +
        ']}'
      ]);
      const out = await decomposeTask(llm, 'x', { summary: 's' });
      expect(out.subtasks.length).toBe(1);
      expect(out.subtasks[0].title).toBe('ok');
    });

    it('returns empty array when LLM response is malformed', async () => {
      const llm = makeMockLLM(['nope']);
      const out = await decomposeTask(llm, 'x', { summary: 's' });
      expect(out.subtasks).toEqual([]);
    });

    it('returns empty array when LLM is not configured', async () => {
      const out = await decomposeTask({ isConfigured: false } as any, 'x', { summary: 's' });
      expect(out.subtasks).toEqual([]);
    });
  });

  describe('formatPlan', () => {
    it('produces a markdown plan card with summary, list, and shortIds', () => {
      const out = formatPlan('Plan a Lisbon trip', [
        { shortId: 'T-101', title: 'Find flights' },
        { shortId: 'T-102', title: 'Book hotel' },
      ]);
      expect(out).toContain('> Plan a Lisbon trip');
      expect(out).toContain('**Plan:**');
      expect(out).toContain('1. `T-101` Find flights');
      expect(out).toContain('2. `T-102` Book hotel');
    });
  });

  // ── Triager-config wiring (v0.3.0) ──────────────────────────────────────
  describe('TriagerConfig', () => {
    it('uses the Triager SOUL.md as the classifier system prompt', async () => {
      const captured: any[] = [];
      const llm = {
        isConfigured: true,
        complete: vi.fn(async (req: any) => {
          captured.push(req);
          return {
            content: '{"isTask": false, "confidence": 0.9, "summary": "casual chat"}',
            model: 'mock', inputTokens: 0, outputTokens: 0, totalTokens: 0, costUsd: 0,
            stopReason: 'end_turn', durationMs: 1,
          };
        }),
      };
      const customSoul = '# Triager\nYou MUST decide.';
      await classifyAsTask(llm as any, 'hi', { soul: customSoul });
      expect(captured.length).toBe(1);
      expect(captured[0].system).toContain('# Triager');
      expect(captured[0].system).toContain('You MUST decide');
    });

    it('honours the Triager modelTier when set', async () => {
      const captured: any[] = [];
      const llm = {
        isConfigured: true,
        complete: vi.fn(async (req: any) => {
          captured.push(req);
          return {
            content: '{"isTask": true, "confidence": 0.85, "summary": "do thing"}',
            model: 'mock', inputTokens: 0, outputTokens: 0, totalTokens: 0, costUsd: 0,
            stopReason: 'end_turn', durationMs: 1,
          };
        }),
      };
      await classifyAsTask(llm as any, 'do thing', { modelTier: 'standard' });
      expect(captured[0].model).toBe('standard');

      // Default is 'fast'
      await classifyAsTask(llm as any, 'do thing', {});
      expect(captured[1].model).toBe('fast');
    });

    it('injects recent feedback as few-shot examples in the user prompt', async () => {
      const captured: any[] = [];
      const llm = {
        isConfigured: true,
        complete: vi.fn(async (req: any) => {
          captured.push(req);
          return {
            content: '{"isTask": true, "confidence": 0.9, "summary": "x"}',
            model: 'mock', inputTokens: 0, outputTokens: 0, totalTokens: 0, costUsd: 0,
            stopReason: 'end_turn', durationMs: 1,
          };
        }),
      };
      await classifyAsTask(llm as any, 'what time is it in Paris', {
        recentFeedback: [
          { message: 'what time is it in Tokyo', was: 'task', shouldBe: 'question' },
          { message: 'schedule a 1:1 with everyone', was: 'question', shouldBe: 'task' },
        ],
      });
      const userMsg = captured[0].messages[0].content;
      expect(userMsg).toContain('Recent user corrections');
      expect(userMsg).toContain('what time is it in Tokyo');
      expect(userMsg).toContain('was TASK, should be QUESTION');
      expect(userMsg).toContain('schedule a 1:1 with everyone');
      expect(userMsg).toContain('was QUESTION, should be TASK');
      // The actual message is still appended after the feedback block.
      expect(userMsg).toContain('what time is it in Paris');
    });

    it('caps feedback at 5 examples even when more are supplied', async () => {
      const captured: any[] = [];
      const llm = {
        isConfigured: true,
        complete: vi.fn(async (req: any) => {
          captured.push(req);
          return {
            content: '{"isTask": false, "confidence": 0.9, "summary": "x"}',
            model: 'mock', inputTokens: 0, outputTokens: 0, totalTokens: 0, costUsd: 0,
            stopReason: 'end_turn', durationMs: 1,
          };
        }),
      };
      const tenFeedback = Array.from({ length: 10 }, (_, i) => ({
        message: `example ${i}`, was: 'task' as const, shouldBe: 'question' as const,
      }));
      await classifyAsTask(llm as any, 'foo', { recentFeedback: tenFeedback });
      const userMsg = captured[0].messages[0].content;
      expect(userMsg).toContain('example 0');
      expect(userMsg).toContain('example 4');
      expect(userMsg).not.toContain('example 5');
      expect(userMsg).not.toContain('example 9');
    });

    it('still accepts a plain SOUL string (legacy callers)', async () => {
      const captured: any[] = [];
      const llm = {
        isConfigured: true,
        complete: vi.fn(async (req: any) => {
          captured.push(req);
          return {
            content: '{"isTask": false, "confidence": 0.5, "summary": "x"}',
            model: 'mock', inputTokens: 0, outputTokens: 0, totalTokens: 0, costUsd: 0,
            stopReason: 'end_turn', durationMs: 1,
          };
        }),
      };
      await classifyAsTask(llm as any, 'hi', 'plain soul string');
      expect(captured[0].system).toContain('plain soul string');
    });
  });
});
