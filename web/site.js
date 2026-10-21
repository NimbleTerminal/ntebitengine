import { BoobaTerminal, installRendererHud, parseRendererFromURL } from './booba/booba.js';

const demos = {
  sprites: {
    title: 'Sprite parade',
    help: 'Space: pause/resume · [/]: robot count · m: glyph/Kitty · +/-: resolution · q: quit.',
    controls: [['Pause / resume', ' '], ['Fewer robots', '['], ['More robots', ']']],
  },
  bounce: {
    title: 'Vector bounce',
    help: 'm: glyph/Kitty · +/-: resolution · q: quit. Resize the browser to change the framebuffer.',
    controls: [],
  },
  text: {
    title: 'Text lab',
    help: 't: scene · f: font · a: alignment · n: sample · [/]: size · Space: pause · m: glyph/Kitty · +/-: resolution · q: quit. Kitty mode makes small text clearer.',
    controls: [['Next scene', 't'], ['Next font', 'f'], ['Alignment', 'a'], ['Next sample', 'n'], ['Smaller type', '['], ['Larger type', ']'], ['Pause / resume', ' ']],
  },
  shaders: {
    title: 'Kage lab',
    help: 't: effect · p: palette · [/]: strength · Space: pause · m: glyph/Kitty · +/-: resolution · q: quit. CRT applies a second shader pass to the plasma image.',
    controls: [['Next effect', 't'], ['Next palette', 'p'], ['Less effect', '['], ['More effect', ']'], ['Pause / resume', ' ']],
  },
  ntui: {
    title: 'ntui Kage lab',
    help: 'Drag panel titles; scroll inside panels. t/p: effect/palette · [/]: strength · Space: pause · m: mode · +/-: resolution · q: quit. Glyph mode and low resolutions use readable terminal controls above the scene. Escape clears text focus.',
    controls: [['Next effect', 't'], ['Next palette', 'p'], ['Less effect', '['], ['More effect', ']'], ['Pause / resume', ' ']],
  },
  loops: {
    title: 'Loop lab',
    help: 'Space: pause simulation · l: slow draw · [/]: speed · r: reset · m: glyph/Kitty · +/-: resolution · q: quit. Read the Guide for event-loop ownership and shutdown.',
    controls: [['Pause simulation', ' '], ['Slow draw', 'l'], ['Slower motion', '['], ['Faster motion', ']'], ['Reset phase', 'r']],
  },
};
const requested = new URLSearchParams(location.search).get('demo');
const demo = Object.hasOwn(demos, requested) ? requested : 'sprites';
const selected = demos[demo];
const status = document.querySelector('#status');
const controls = document.querySelector('#controls');
document.querySelector('#title').textContent = selected.title;
document.querySelector('#help').textContent = selected.help;
document.querySelector('#command').textContent = `go run ./examples/${demo}`;
document.title = `${selected.title} · ntebitengine`;
const restart = new URL(location.href);
restart.searchParams.set('demo', demo);
document.querySelector('#restart').href = restart.href;
for (const link of document.querySelectorAll('[data-demo]')) {
  if (link.dataset.demo === demo) link.setAttribute('aria-current', 'page');
}

let terminal;
let ended = false;
function setControlsEnabled(enabled) {
  for (const button of controls.querySelectorAll('button')) button.disabled = !enabled;
}
function failed(error) {
  ended = true;
  setControlsEnabled(false);
  status.dataset.error = '';
  status.textContent = `Could not run the demo: ${error.message}. Check WebGL support or try Restart.`;
  console.error(error);
}

// Ebitengine appends its own canvas directly to body during Go initialization.
// Only that canvas is moved offscreen; booba's nested terminal canvas stays visible.
function hideGPUCanvas() {
  for (const canvas of document.querySelectorAll('body > canvas')) {
    canvas.tabIndex = -1;
    canvas.setAttribute('aria-hidden', 'true');
  }
}
const canvasObserver = new MutationObserver(hideGPUCanvas);
canvasObserver.observe(document.body, { childList: true });

async function main() {
  for (const [label, key] of [...selected.controls, ['Glyph / Kitty', 'm'], ['Softer', '-'], ['Sharper', '+'], ['Quit', 'q']]) {
    const button = document.createElement('button');
    button.type = 'button';
    button.textContent = label;
    button.disabled = true;
    button.addEventListener('click', () => {
      if (ended || !terminal?.term) return;
      terminal.term.input(key, true);
      terminal.focus();
    });
    controls.append(button);
  }
  // Open the terminal before starting Go, so the GPU canvas never takes focus.
  terminal = new BoobaTerminal('terminal', { renderer: parseRendererFromURL() });
  await terminal.init();
  const go = new Go();
  const response = await fetch(`demos/${demo}.wasm`);
  if (!response.ok) throw new Error(`Download failed (${response.status})`);
  const bytes = await response.arrayBuffer();
  const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
  let exitCode = 0;
  go.exit = code => { exitCode = code; };
  const running = go.run(instance);
  running.then(() => {
    ended = true;
    setControlsEnabled(false);
    if (exitCode !== 0) {
      failed(new Error(`Go exited with status ${exitCode}`));
    } else {
      status.textContent = 'Session ended. Choose Restart to run again.';
    }
  }, failed);
  // Go may yield during GPU initialization before booba installs its bridge.
  const deadline = performance.now() + 15000;
  while (typeof globalThis.bubbletea_read !== 'function') {
    if (ended) return;
    if (performance.now() > deadline) throw new Error('Terminal startup timed out');
    await new Promise(resolve => setTimeout(resolve, 20));
  }
  if (ended) return;
  hideGPUCanvas();
  terminal.connectWasm();
  terminal.focus();
  installRendererHud(terminal.term, {
    parent: document.querySelector('.terminal-frame'),
    position: 'absolute',
    bindToggleHotkey: false,
  });
  setControlsEnabled(true);
  status.textContent = 'Running locally in your browser. No terminal server required.';
}
window.addEventListener('pagehide', () => {
  canvasObserver.disconnect();
  terminal?.dispose();
}, { once: true });
main().catch(failed);
