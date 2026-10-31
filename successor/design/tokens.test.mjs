import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { crewSlot, loadTokens, paths, swift, withTokens } from './tokens.mjs';
import { crewSlot as webCrewSlot } from '../web/transcript.js';

// tokens.json is the one source for the skin; the web stylesheet and FortTheme.swift are built from it.
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

test('text on every fill is readable (WCAG AA for body text)', () => {
  const luminance = hex => {
    const [r, g, b] = [1, 3, 5].map(index => parseInt(hex.slice(index, index + 2), 16) / 255).map(c => c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const contrast = (a, b) => { const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const { color } = tokens;
  for (const mode of ['light', 'dark']) {
    for (const ground of ['paper', 'panel', 'bar', 'caption', 'mine']) {
      assert.ok(contrast(color.ink[mode], color[ground][mode]) >= 4.5, `ink on ${ground} (${mode})`);
      assert.ok(contrast(color.muted[mode], color[ground][mode]) >= 4.5, `muted on ${ground} (${mode})`);
    }
    for (const fill of ['accent', 'working', 'done', 'needsYou']) assert.ok(contrast(color.captionInk[mode], color[fill][mode]) >= 4.5, `captionInk on ${fill} (${mode})`);
    assert.ok(contrast('#FFFFFF', color.failed[mode]) >= 3, `white on failed (${mode}, large bold text)`);
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
