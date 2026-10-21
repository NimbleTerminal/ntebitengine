// Validate the generated GitHub Pages artifact without launching a browser.
import assert from 'node:assert/strict';
import { readFile, access } from 'node:fs/promises';

const demos = ['bounce', 'sprites', 'text'];
for (const file of [
  'index.html', 'site.css', 'site.js', 'wasm_exec.js', 'booba/booba.js',
  'ghostty-web/ghostty-web.js', 'ghostty-web/ghostty-vt.wasm',
]) await access(`web/${file}`);

for (const demo of demos) {
  const bytes = await readFile(`web/demos/${demo}.wasm`);
  assert.ok(WebAssembly.validate(bytes), `${demo}: valid WASM`);
}
const html = await readFile('web/index.html', 'utf8');
for (const [, path] of html.matchAll(/(?:src|href)="([^"?#]+)"/g)) {
  if (/^(https?:|\.\/)/.test(path)) continue;
  await access(`web/${path}`);
}
for (const demo of demos) assert.ok(html.includes(`data-demo="${demo}"`), `${demo}: navigation`);
console.log('Site links, runtime assets, and all WASM binaries checked.');
