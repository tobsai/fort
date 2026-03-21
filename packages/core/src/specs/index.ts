/**
 * Spec System — Spec-Driven Development Cycle
 *
 * All development follows: spec → implement → verify → merge/rollback.
 * Specs live in the specs/ directory as machine-readable markdown.
 */

import { readFileSync, writeFileSync, readdirSync, existsSync, mkdirSync } from 'node:fs';
import { join, basename } from 'node:path';
import { v4 as uuid } from 'uuid';
import type { Spec, SpecStatus } from '../types.js';

const SPEC_TEMPLATE = `# {{title}}

**ID:** {{id}}
**Status:** {{status}}
**Author:** {{author}}
**Created:** {{createdAt}}

## Goal

{{goal}}

## Approach

{{approach}}

## Affected Files

{{affectedFiles}}

## Test Criteria

{{testCriteria}}

## Rollback Plan

{{rollbackPlan}}
`;

export class SpecManager {
  private specsDir: string;

  constructor(specsDir: string) {
    this.specsDir = specsDir;
    if (!existsSync(specsDir)) {
      mkdirSync(specsDir, { recursive: true });
    }
  }

  create(params: {
    title: string;
    goal: string;
    approach: string;
    affectedFiles: string[];
    testCriteria: string[];
    rollbackPlan: string;
    author?: string;
  }): Spec {
    const spec: Spec = {
      id: uuid(),
      title: params.title,
      status: 'draft',
      goal: params.goal,
      approach: params.approach,
      affectedFiles: params.affectedFiles,
      testCriteria: params.testCriteria,
      rollbackPlan: params.rollbackPlan,
      createdAt: new Date(),
      updatedAt: new Date(),
      author: params.author ?? 'fort',
    };

    this.saveSpec(spec);
    return spec;
  }

  get(specId: string): Spec | null {
    const filePath = this.specFilePath(specId);
    if (!existsSync(filePath)) return null;
    return this.parseSpec(readFileSync(filePath, 'utf-8'), specId);
  }

  updateStatus(specId: string, status: SpecStatus): Spec | null {
    const spec = this.get(specId);
    if (!spec) return null;
    spec.status = status;
    spec.updatedAt = new Date();
    this.saveSpec(spec);
    return spec;
  }

  list(status?: SpecStatus): Spec[] {
    if (!existsSync(this.specsDir)) return [];

    const files = readdirSync(this.specsDir).filter((f) => f.endsWith('.md'));
    const specs: Spec[] = [];

    for (const file of files) {
      const content = readFileSync(join(this.specsDir, file), 'utf-8');
      const id = basename(file, '.md');
      const spec = this.parseSpec(content, id);
      if (spec && (!status || spec.status === status)) {
        specs.push(spec);
      }
    }

    return specs.sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime());
  }

  private saveSpec(spec: Spec): void {
    const content = SPEC_TEMPLATE
      .replace('{{title}}', spec.title)
      .replace('{{id}}', spec.id)
      .replace('{{status}}', spec.status)
      .replace('{{author}}', spec.author)
      .replace('{{createdAt}}', spec.createdAt.toISOString())
      .replace('{{goal}}', spec.goal)
      .replace('{{approach}}', spec.approach)
      .replace('{{affectedFiles}}', spec.affectedFiles.map((f) => `- ${f}`).join('\n'))
      .replace('{{testCriteria}}', spec.testCriteria.map((c) => `- [ ] ${c}`).join('\n'))
      .replace('{{rollbackPlan}}', spec.rollbackPlan);

    writeFileSync(this.specFilePath(spec.id), content, 'utf-8');
  }

  private specFilePath(specId: string): string {
    return join(this.specsDir, `${specId}.md`);
  }

  private parseSpec(content: string, id: string): Spec | null {
    try {
      const title = content.match(/^# (.+)$/m)?.[1] ?? 'Untitled';
      const status = (content.match(/\*\*Status:\*\* (.+)$/m)?.[1] ?? 'draft') as SpecStatus;
      const author = content.match(/\*\*Author:\*\* (.+)$/m)?.[1] ?? 'unknown';
      const createdAt = content.match(/\*\*Created:\*\* (.+)$/m)?.[1] ?? new Date().toISOString();

      const getSection = (heading: string): string => {
        const regex = new RegExp(`## ${heading}\\n\\n([\\s\\S]*?)(?=\\n## |$)`);
        return regex.exec(content)?.[1]?.trim() ?? '';
      };

      const affectedFiles = getSection('Affected Files')
        .split('\n')
        .map((l) => l.replace(/^- /, '').trim())
        .filter(Boolean);

      const testCriteria = getSection('Test Criteria')
        .split('\n')
        .map((l) => l.replace(/^- \[[ x]\] /, '').trim())
        .filter(Boolean);

      return {
        id,
        title,
        status,
        goal: getSection('Goal'),
        approach: getSection('Approach'),
        affectedFiles,
        testCriteria,
        rollbackPlan: getSection('Rollback Plan'),
        createdAt: new Date(createdAt),
        updatedAt: new Date(),
        author,
      };
    } catch {
      return null;
    }
  }
}
