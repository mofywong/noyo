#!/usr/bin/env node
// Post-build assertions. `findstr` cannot be trusted here: minified bundles are a
// single multi-megabyte line, which it skips. These checks are the difference
// between "rebuilt" and "rebuilt and actually shipped the new code".
// Usage: node scripts/check-artifacts.cjs goview-5ab05b4-noyo22
const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const engine = process.argv[2];
if (!engine) { console.error('usage: node scripts/check-artifacts.cjs <engineBuildId>'); process.exit(2); }

const dist = path.join(root, 'backend/dist');
const failures = [];
function check(label, fn) {
  try { const detail = fn(); console.log('  ok   ' + label + (detail ? '  ' + detail : '')); }
  catch (error) { failures.push(label + ': ' + error.message); console.log('  FAIL ' + label + '  ' + error.message); }
}
function readAll(dir, extension) {
  if (!fs.existsSync(dir)) throw new Error('missing directory ' + dir);
  return fs.readdirSync(dir).filter(file => file.endsWith(extension)).map(file => fs.readFileSync(path.join(dir, file), 'utf8'));
}

check('engine index ' + engine, () => {
  const file = path.join(dist, 'screen-editor', engine, 'index.html');
  if (!fs.existsSync(file)) throw new Error('not found');
  return '';
});

check('engine manifest pinned to ' + engine, () => {
  const manifest = JSON.parse(fs.readFileSync(path.join(dist, 'screen-editor/manifest.json'), 'utf8'));
  if (manifest.buildId !== engine) throw new Error('manifest says ' + manifest.buildId);
  return '';
});

check('floating panel css present', () => {
  const css = readAll(path.join(dist, 'screen-editor', engine, 'assets'), '.css').join('\n');
  for (const token of ['ioc-assistant-moving', 'ioc-assistant-resize', 'is-corner']) {
    if (!css.includes(token)) throw new Error('missing "' + token + '"');
  }
  const fixedRule = css.split('}').find(rule => rule.includes('ioc-assistant') && rule.includes('position:fixed'));
  if (!fixedRule) throw new Error('no position:fixed rule for the assistant');
  return '';
});

check('host bundle pinned to ' + engine, () => {
  const bundle = readAll(path.join(dist, 'assets'), '.js');
  if (!bundle.some(text => text.includes(engine))) throw new Error('SCREEN_ENGINE not found in any js chunk');
  return '';
});

check('assistant title source localised', () => {
  const bundle = readAll(path.join(dist, 'assets'), '.js').join('\n');
  if (!bundle.includes('llm_source_ioc_screen_title')) throw new Error('i18n key missing');
  return '';
});

if (failures.length) {
  console.error('\n' + failures.length + ' artifact check(s) failed.');
  process.exit(1);
}
console.log('\nAll artifact checks passed for ' + engine);
