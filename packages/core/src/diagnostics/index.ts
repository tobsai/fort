/**
 * Diagnostics — Fort Doctor
 *
 * Comprehensive health check system. Output is both human-readable
 * and machine-readable so Fort can parse its own health.
 */

import type { DiagnosticResult, DiagnosticCheck } from '../types.js';

export interface DiagnosticsProvider {
  diagnose(): DiagnosticResult | Promise<DiagnosticResult>;
}

export class FortDoctor {
  private providers: Map<string, DiagnosticsProvider> = new Map();

  register(name: string, provider: DiagnosticsProvider): void {
    this.providers.set(name, provider);
  }

  async runAll(): Promise<DiagnosticResult[]> {
    const results: DiagnosticResult[] = [];

    for (const [name, provider] of this.providers) {
      try {
        const result = await provider.diagnose();
        results.push(result);
      } catch (err) {
        results.push({
          module: name,
          status: 'unhealthy',
          checks: [{
            name: 'Module diagnostics',
            passed: false,
            message: `Diagnostics failed: ${err instanceof Error ? err.message : String(err)}`,
          }],
        });
      }
    }

    return results;
  }

  async runModule(name: string): Promise<DiagnosticResult | null> {
    const provider = this.providers.get(name);
    if (!provider) return null;
    return provider.diagnose();
  }

  getRegisteredModules(): string[] {
    return Array.from(this.providers.keys());
  }

  static summarize(results: DiagnosticResult[]): {
    overall: 'healthy' | 'degraded' | 'unhealthy';
    totalChecks: number;
    passed: number;
    failed: number;
    modules: Record<string, 'healthy' | 'degraded' | 'unhealthy'>;
  } {
    const modules: Record<string, 'healthy' | 'degraded' | 'unhealthy'> = {};
    let totalChecks = 0;
    let passed = 0;
    let failed = 0;

    for (const result of results) {
      modules[result.module] = result.status;
      for (const check of result.checks) {
        totalChecks++;
        if (check.passed) passed++;
        else failed++;
      }
    }

    let overall: 'healthy' | 'degraded' | 'unhealthy' = 'healthy';
    if (Object.values(modules).some((s) => s === 'unhealthy')) overall = 'unhealthy';
    else if (Object.values(modules).some((s) => s === 'degraded')) overall = 'degraded';

    return { overall, totalChecks, passed, failed, modules };
  }
}
