// Builds Fort's comic skin from tokens.json into the two places that use it: the :root block of the
// web client's app.css and FortKit's FortTheme.swift. `--write` regenerates both; tokens.test.mjs
// fails when either has drifted from the JSON.
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const here = path => fileURLToPath(new URL(path, import.meta.url));
export const paths = { tokens: here('./tokens.json'), css: here('../web/app.css'), swift: here('../apple/FortKit/Sources/FortKit/FortTheme.swift') };
export const loadTokens = () => JSON.parse(readFileSync(paths.tokens, 'utf8'));
const START = '/* fort-tokens:start', END = '/* fort-tokens:end */';
const kebab = name => name.replace(/[A-Z]/g, letter => '-' + letter.toLowerCase());

/** The generated :root block, light values first and the dark ones that differ under prefers-color-scheme. */
export function css(tokens) {
  const colors = Object.entries(tokens.color);
  const light = colors.map(([name, value]) => `--${kebab(name)}: ${value.light};`);
  const dark = colors.filter(([, value]) => value.dark !== value.light).map(([name, value]) => `--${kebab(name)}: ${value.dark};`);
  const { size, font } = tokens;
  return [
    `${START} · generated from successor/design/tokens.json by \`node successor/design/tokens.mjs --write\` */`,
    ':root {',
    '  ' + light.join(' '),
    '  ' + tokens.crew.map((value, index) => `--crew-${index}: ${value};`).join(' '),
    `  --sky: linear-gradient(100deg, ${tokens.sky.map((value, index) => `${value} ${Math.round(index * 100 / (tokens.sky.length - 1))}%`).join(', ')});`,
    `  --stroke: ${size.stroke}px; --radius: ${size.radius}px; --bubble-radius: ${size.bubbleRadius}px; --shadow: ${size.shadow}px; --touch: ${size.touch}px;`,
    `  --font-display: ${font.display.css}; --font-display-weight: ${font.display.weight}; --font-body: ${font.body.css};`,
    '}',
    '@media (prefers-color-scheme: dark) {',
    '  :root { ' + dark.join(' ') + ' }',
    '}',
    END
  ].join('\n');
}

/** app.css with its token block replaced; everything outside the markers is kept as written. */
export function withTokens(stylesheet, tokens) {
  const start = stylesheet.indexOf(START), end = stylesheet.indexOf(END);
  if (start < 0 || end < start) throw new Error('app.css has no fort-tokens block');
  return stylesheet.slice(0, start) + css(tokens) + stylesheet.slice(end + END.length);
}

const hex = value => '0x' + value.slice(1).toUpperCase();
/** FortTheme.swift, whole. */
export function swift(tokens) {
  const colors = Object.entries(tokens.color).map(([name, value]) => [
    `    /// ${value.use}`,
    `    public static let ${name} = ${value.light === value.dark ? `Color(hex:${hex(value.light)})` : `Color(light:${hex(value.light)},dark:${hex(value.dark)})`}`
  ].join('\n'));
  const { size } = tokens;
  return [
    '// Generated from successor/design/tokens.json by `node successor/design/tokens.mjs --write`. Edit the JSON, not this file.',
    'import SwiftUI',
    '',
    "/// Fort's comic skin. The helpers that draw with it are in FortThemeSupport.swift.",
    'public enum FortTheme {',
    ...colors,
    '    /// Portrait colours for the crew; FortTheme.crewColor(for:) picks one per Agent.',
    `    public static let crew: [Color] = [${tokens.crew.map(value => `Color(hex:${hex(value)})`).join(',')}]`,
    '    /// The golden-hour sky behind the top bar, left to right.',
    `    public static let sky: [Color] = [${tokens.sky.map(value => `Color(hex:${hex(value)})`).join(',')}]`,
    `    public static let stroke: CGFloat = ${size.stroke}`,
    `    public static let radius: CGFloat = ${size.radius}`,
    `    public static let bubbleRadius: CGFloat = ${size.bubbleRadius}`,
    `    public static let shadow: CGFloat = ${size.shadow}`,
    `    public static let touch: CGFloat = ${size.touch}`,
    `    public static let displayFont = "${tokens.font.display.apple}"`,
    '}',
    ''
  ].join('\n');
}

/** The crew slot for an Agent: FNV-1a over its id's UTF-8 bytes, so every surface gives it the same colour. */
export function crewSlot(agentId, count) {
  let hash = 0x811c9dc5;
  for (const byte of new TextEncoder().encode(agentId)) hash = Math.imul(hash ^ byte, 0x01000193) >>> 0;
  return hash % count;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  if (!process.argv.includes('--write')) throw new Error('Run with --write to regenerate app.css and FortTheme.swift');
  const tokens = loadTokens();
  writeFileSync(paths.css, withTokens(readFileSync(paths.css, 'utf8'), tokens));
  writeFileSync(paths.swift, swift(tokens));
}
