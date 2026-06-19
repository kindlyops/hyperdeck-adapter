import './style.css';
import { wireDownloadButtons } from './downloads';

// ---------------------------------------------------------------------------
// Download buttons: resolve latest release assets right away (cheap fetch).
// ---------------------------------------------------------------------------
wireDownloadButtons();

// ---------------------------------------------------------------------------
// Hero scene: lazy-init after first paint so the page is interactive first.
// ---------------------------------------------------------------------------
function webglAvailable(): boolean {
  try {
    const canvas = document.createElement('canvas');
    return Boolean(
      window.WebGLRenderingContext &&
        (canvas.getContext('webgl2') || canvas.getContext('webgl')),
    );
  } catch {
    return false;
  }
}

function showFallback(): void {
  const fallback = document.getElementById('hero-fallback');
  if (fallback) fallback.hidden = false;
}

function initHero(): void {
  const hero = document.getElementById('hero');
  if (!hero) return;
  if (!webglAvailable()) {
    showFallback();
    return;
  }
  import('./scene')
    .then(({ startControlRoom }) => startControlRoom(hero))
    .catch(() => showFallback());
}

requestAnimationFrame(() => {
  if (typeof window.requestIdleCallback === 'function') {
    window.requestIdleCallback(initHero, { timeout: 1500 });
  } else {
    setTimeout(initHero, 50);
  }
});

// ---------------------------------------------------------------------------
// Easter egg trigger: type "td" (technical director). Module is lazy-loaded.
// ---------------------------------------------------------------------------
let eggBusy = false;
let eggController: { dismiss(): void } | null = null;
let typed = '';

async function triggerEgg(): Promise<void> {
  if (eggBusy) return;
  if (eggController) {
    eggController.dismiss();
    return;
  }
  eggBusy = true;
  try {
    const { runEasterEgg } = await import('./easterEgg');
    eggController = await runEasterEgg(() => {
      eggController = null;
    });
  } catch (err) {
    console.warn('easter egg failed to load', err);
  } finally {
    eggBusy = false;
  }
}

window.addEventListener('keydown', (ev: KeyboardEvent) => {
  const target = ev.target as HTMLElement | null;
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) {
    return;
  }

  if (ev.key === 'Escape' && eggController) {
    eggController.dismiss();
    return;
  }

  if (ev.key.length === 1) {
    typed = (typed + ev.key.toLowerCase()).slice(-2);
    if (typed === 'td') {
      typed = '';
      void triggerEgg();
    }
  }
});
