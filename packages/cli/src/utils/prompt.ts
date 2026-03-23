/**
 * Interactive CLI prompts — no external dependencies.
 * Uses raw stdin for arrow-key selection and readline for text input.
 */

import { createInterface } from 'node:readline';
import { bold, dim, cyan, green } from './format.js';

/**
 * Prompt for free-form text input.
 */
export async function promptText(
  question: string,
  defaultValue?: string,
): Promise<string> {
  const rl = createInterface({ input: process.stdin, output: process.stdout });
  const suffix = defaultValue ? dim(` (${defaultValue})`) : '';

  return new Promise((resolve) => {
    rl.question(`  ${question}${suffix} `, (answer) => {
      rl.close();
      const trimmed = answer.trim();
      resolve(trimmed || defaultValue || '');
    });
  });
}

/**
 * Prompt to select one option from a list using arrow keys.
 */
export async function promptSelect(
  question: string,
  options: { label: string; value: string; preview?: string }[],
): Promise<string> {
  return new Promise((resolve) => {
    let selected = 0;

    const render = () => {
      // Move cursor up to overwrite previous render
      if (rendered) {
        process.stdout.write(`\x1b[${totalLines}A`);
      }

      process.stdout.write(`  ${bold(question)}\n`);
      for (let i = 0; i < options.length; i++) {
        const prefix = i === selected ? cyan('❯ ') : '  ';
        const label = i === selected ? bold(options[i].label) : options[i].label;
        process.stdout.write(`    ${prefix}${label}\n`);
      }

      // Show preview if available
      const preview = options[selected].preview;
      if (preview) {
        const previewLines = preview.split('\n');
        process.stdout.write('\n');
        for (const line of previewLines) {
          process.stdout.write(`      ${dim(line)}\n`);
        }
        totalLines = 1 + options.length + 1 + previewLines.length;
      } else {
        totalLines = 1 + options.length;
      }

      rendered = true;
    };

    let rendered = false;
    let totalLines = 0;

    process.stdin.setRawMode(true);
    process.stdin.resume();
    process.stdin.setEncoding('utf-8');

    render();

    const onData = (key: string) => {
      if (key === '\x1b[A' || key === 'k') {
        // Up arrow or k
        selected = (selected - 1 + options.length) % options.length;
        render();
      } else if (key === '\x1b[B' || key === 'j') {
        // Down arrow or j
        selected = (selected + 1) % options.length;
        render();
      } else if (key === '\r' || key === '\n') {
        // Enter
        process.stdin.setRawMode(false);
        process.stdin.pause();
        process.stdin.removeListener('data', onData);
        process.stdout.write('\n');
        resolve(options[selected].value);
      } else if (key === '\x03') {
        // Ctrl+C
        process.stdin.setRawMode(false);
        process.exit(0);
      }
    };

    process.stdin.on('data', onData);
  });
}

/**
 * Prompt for an emoji character with suggestions.
 */
export async function promptEmoji(
  question: string,
  suggestions: string[] = ['🏰', '🤖', '🦉', '🧠', '⚡', '🛡️', '🔮', '🌟', '🐙', '🎯'],
): Promise<string> {
  console.log(`  ${bold(question)}`);
  console.log(`    ${dim('Suggestions:')} ${suggestions.join('  ')}`);
  const answer = await promptText('  Enter emoji:');
  return answer || suggestions[0];
}
