import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { crewSlot, loadTokens, paths, swift, withTokens } from './tokens.mjs';
import { crewSlot as webCrewSlot } from '../web/transcript.js';

// tokens.json is the one source for the comic skin; the web stylesheet and FortTheme.swift are built from it.
const tokens = loadTokens();

test('the web stylesheet carries the current tokens', () => {
  const stylesheet = readFileSync(paths.css, 'utf8');
  assert.equal(stylesheet, withTokens(stylesheet, tokens), 'Run node successor/design/tokens.mjs --write');
});

test('FortTheme.swift carries the current tokens', () => {
  assert.equal(readFileSync(paths.swift, 'utf8'), swift(tokens), 'Run node successor/design/tokens.mjs --write');
});

test('every colour has a light and a dark value and says what it is for', () => {
  for (const [name, value] of Object.entries(tokens.color)) {
    assert.match(value.light, /^#[0-9A-F]{6}$/, name);
    assert.match(value.dark, /^#[0-9A-F]{6}$/, name);
    assert.ok(value.use, name);
  }
  assert.equal(tokens.color.needsYou.light, tokens.color.accent.light, 'what needs you is quest gold');
});

const luminance = hex => {
  const [r, g, b] = [1, 3, 5].map(index => parseInt(hex.slice(index, index + 2), 16) / 255).map(c => c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};
const contrast = (a, b) => { const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };

test('text on the comic fills is readable (WCAG AA for body text)', () => {
  const { color } = tokens;
  for (const mode of ['light', 'dark']) {
    for (const ground of ['paper', 'panel', 'bar']) {
      assert.ok(contrast(color.ink[mode], color[ground][mode]) >= 4.5, `ink on ${ground} (${mode})`);
      assert.ok(contrast(color.muted[mode], color[ground][mode]) >= 4.5, `muted on ${ground} (${mode})`);
    }
    for (const fill of ['caption', 'accent', 'working', 'done', 'needsYou']) assert.ok(contrast(color.captionInk[mode], color[fill][mode]) >= 4.5, `captionInk on ${fill} (${mode})`);
    assert.ok(contrast('#FFFFFF', color.failed[mode]) >= 3, `white on failed (${mode}, large bold text)`);
  }
});

test('the web bars that pick their own fill keep readable text in light and noir', () => {
  // The sky gradient and the status strips are fixed fills, so the text the stylesheet pairs with them
  // has to hold up in both modes, not only in the one it was drawn in.
  const sheet = readFileSync(paths.css, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').replace(/@media[^{]*\{(?:[^{}]*\{[^}]*\})*\s*\}/g, '');
  const rules = [...sheet.matchAll(/([^{}]+)\{([^}]*)\}/g)].map(([, selectors, body]) => ({
    selectors: selectors.split(',').map(item => item.trim()),
    declarations: Object.fromEntries(body.split(';').map(item => item.split(':').map(part => part.trim())).filter(([key, value]) => key && value))
  }));
  const declared = (selector, properties) => rules.filter(rule => rule.selectors.includes(selector)).flatMap(rule => properties.map(property => rule.declarations[property]).filter(Boolean)).at(-1);
  const resolve = (value, mode) => {
    const variable = value?.match(/^var\(--([a-z-]+)\)$/)?.[1];
    if (!variable) return value;
    return tokens.color[variable.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase())]?.[mode];
  };
  for (const selector of ['.top .caption', '.top button', '.strip', '.strip.needsYou', '.strip.failed', '.strip.working']) {
    const fill = declared(selector, ['background', 'background-color']);
    const text = declared(selector, ['color']);
    assert.ok(fill && text, `${selector} sets both its fill and its text colour`);
    for (const mode of ['light', 'dark']) {
      const [ground, ink] = [resolve(fill, mode), resolve(text, mode)];
      assert.match(`${ground} ${ink}`, /^#[0-9A-F]{6} #[0-9A-F]{6}$/i, `${selector} resolves to plain colours (${mode})`);
      assert.ok(contrast(ink, ground) >= 4.5, `${selector}: ${ink} on ${ground} is ${contrast(ink, ground).toFixed(2)}:1 (${mode})`);
    }
  }
});

test('an Agent gets the same crew colour on the web as in the apps', () => {
  // The same vectors are in FortKit's FortThemeTests, so Swift and JavaScript must agree.
  const vectors = [['', 1], ['agent-1', 4], ['Lewis', 3], ['b6f1c2d0-5a4e-4c1b-9f3e-2d7a8c9e0f11', 2], ['émile', 2]];
  for (const [id, slot] of vectors) {
    assert.equal(crewSlot(id, tokens.crew.length), slot, id);
    assert.equal(webCrewSlot(id), slot, id);
  }
});
