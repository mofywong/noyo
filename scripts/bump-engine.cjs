#!/usr/bin/env node
// Bump the screen adapter generation in the three places that must agree:
//   integrations/goview/UPSTREAM.json   (source of truth for the build id)
//   backend/core/screen/document.go     (EngineBuildID + legacy map)
//   frontend/src/screens/api.js         (SCREEN_ENGINE handed to the host)
// Usage: node scripts/bump-engine.cjs goview-5ab05b4-noyo22
const fs = require('fs');
const path = require('path');

const root = path.resolve(__dirname, '..');
const next = process.argv[2];
if (!/^goview-[0-9a-f]+-noyo\d+$/.test(next || '')) {
  console.error('usage: node scripts/bump-engine.cjs goview-<sha7>-noyo<nn>');
  process.exit(2);
}

const upstreamPath = path.join(root, 'integrations/goview/UPSTREAM.json');
const upstream = JSON.parse(fs.readFileSync(upstreamPath, 'utf8'));
const prev = upstream.buildId;
if (prev === next) {
  console.log('already at ' + next);
  process.exit(0);
}

upstream.buildId = next;
upstream.adapterVersion = (Number(upstream.adapterVersion) || 0) + 1;
upstream.previousBuildIds = Array.from(new Set([].concat(upstream.previousBuildIds || [], [prev])));
fs.writeFileSync(upstreamPath, JSON.stringify(upstream, null, 2) + '\n', 'utf8');

const docPath = path.join(root, 'backend/core/screen/document.go');
let doc = fs.readFileSync(docPath, 'utf8');
const pinned = new RegExp('EngineBuildID\\s*=\\s*"' + prev + '"');
if (!pinned.test(doc)) throw new Error('EngineBuildID "' + prev + '" not found in document.go');
doc = doc.replace(pinned, 'EngineBuildID    = "' + next + '"');
if (!doc.includes('"' + prev + '": true')) {
  doc = doc.replace(/(var LegacyEngineBuildIDs = map\[string\]bool\{)([\s\S]*?)(\})/, (all, head, body, tail) => head + body + ', "' + prev + '": true' + tail);
}
fs.writeFileSync(docPath, doc, 'utf8');

// Anchored on the SCREEN_ENGINE declaration: a bare global replace would hit
// whichever occurrence of the id happens to come first in the file.
const apiPath = path.join(root, 'frontend/src/screens/api.js');
let api = fs.readFileSync(apiPath, 'utf8');
const engineLine = new RegExp('(SCREEN_ENGINE\\s*=\\s*[\'"])([^\'"]+)([\'"])');
const engineMatch = api.match(engineLine);
if (!engineMatch) throw new Error('SCREEN_ENGINE declaration not found in api.js');
if (engineMatch[2] !== prev) throw new Error('SCREEN_ENGINE is "' + engineMatch[2] + '", expected "' + prev + '" -- refusing to guess');
api = api.replace(engineLine, '$1' + next + '$3');
fs.writeFileSync(apiPath, api, 'utf8');

console.log(prev + ' -> ' + next);
