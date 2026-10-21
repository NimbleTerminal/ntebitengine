// Validate the generated GitHub Pages artifact without launching a browser.
import assert from 'node:assert/strict';
import { readFile, access, readdir, stat } from 'node:fs/promises';

const demos = ['bounce', 'sprites', 'text', 'shaders', 'loops'];
for (const file of [
  'index.html', 'site.css', 'site.js', 'wasm_exec.js', 'booba/booba.js',
  'ghostty-web/ghostty-web.js', 'ghostty-web/ghostty-vt.wasm',
  'docs/index.html', 'docs/guide/event-loops/index.html', 'docs/guide/loop-lab/index.html',
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
// Hugo's minifier can omit attribute quotes. Check every guide page so broken
// theme paths or subdirectory navigation fail the build, not the reader.
for (const file of await readdir('web/docs', { recursive: true })) {
  if (!file.endsWith('.html')) continue;
  const page = await readFile(`web/docs/${file}`, 'utf8');
  const base = new URL(`docs/${file}`, 'https://local.invalid/');
  for (const [, quoted, plain] of page.matchAll(/(?:href|src)=(?:"([^"]+)"|([^\s>]+))/g)) {
    const url = new URL(quoted ?? plain, base);
    if (url.origin !== base.origin) continue;
    let path = `web${decodeURIComponent(url.pathname)}`;
    const info = await stat(path);
    if (info.isDirectory()) path += `${path.endsWith('/') ? '' : '/'}index.html`;
    await access(path);
  }
}
console.log('Site and guide links, runtime assets, and all WASM binaries checked.');
