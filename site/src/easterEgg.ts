// The easter egg (type "td"): the page is grabbed as a still, then wiped away
// with a retro Amiga Video Toaster-style DVE — the frame shatters into a grid of
// tiles that flip and fly toward you in a diagonal sweep, revealing a levitating
// museum-piece sculpture of the adapter's hexagonal architecture. Lazy-loaded.

import html2canvas from 'html2canvas';
import {
  AmbientLight,
  BoxGeometry,
  BufferAttribute,
  BufferGeometry,
  CanvasTexture,
  Color,
  CylinderGeometry,
  DirectionalLight,
  EdgesGeometry,
  Group,
  Line,
  LineBasicMaterial,
  LineSegments,
  Mesh,
  MeshBasicMaterial,
  MeshPhongMaterial,
  PerspectiveCamera,
  PlaneGeometry,
  Scene,
  Sprite,
  SpriteMaterial,
  SRGBColorSpace,
  TorusGeometry,
  Vector3,
  WebGLRenderer,
  DoubleSide,
} from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';

const BG = 0x0a0c10;
const GREEN = 0x2ecc71;
const MINT = 0xa6f4c5;
const CYAN = 0x2bd4ff;
const AMBER = 0xffb347;
const SLATE = 0xcfe3ff;
const PANEL = 0x161c25;

const TILE_COLS = 6;
const TILE_ROWS = 4;
const WIPE_SPREAD = 1.35; // stagger window across the diagonal (s)
const WIPE_DUR = 0.95; // per-tile flip duration (s)

export interface EggController {
  dismiss(): void;
}

function makeLabelSprite(text: string, options?: { small?: boolean; color?: string }): Sprite {
  const small = options?.small ?? false;
  const fontPx = small ? 30 : 44;
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;
  ctx.font = `${fontPx}px 'JetBrains Mono', Consolas, monospace`;
  const metrics = ctx.measureText(text);
  canvas.width = Math.ceil(metrics.width) + 24;
  canvas.height = fontPx + 20;
  ctx.font = `${fontPx}px 'JetBrains Mono', Consolas, monospace`;
  ctx.textBaseline = 'middle';
  ctx.textAlign = 'center';
  ctx.fillStyle = options?.color ?? '#cfe3ff';
  ctx.fillText(text, canvas.width / 2, canvas.height / 2);

  const texture = new CanvasTexture(canvas);
  texture.colorSpace = SRGBColorSpace;
  const sprite = new Sprite(new SpriteMaterial({ map: texture, transparent: true, depthWrite: false }));
  const scale = small ? 0.0044 : 0.0056;
  sprite.scale.set(canvas.width * scale, canvas.height * scale, 1);
  return sprite;
}

// ---------------------------------------------------------------------------
// Toaster DVE: a grid of tiles textured from a page snapshot, flipping and
// flying off in a diagonal sweep. One shared texture (per-tile UVs), so it
// stays cheap. Drawn over the sculpture, which is revealed as tiles depart.
// ---------------------------------------------------------------------------
interface Wipe {
  scene: Scene;
  camera: PerspectiveCamera;
  /** Advance the sweep; returns true while tiles are still on screen. */
  step(elapsed: number): boolean;
  dispose(): void;
}

function smoothstep(x: number): number {
  if (x <= 0) return 0;
  if (x >= 1) return 1;
  return x * x * (3 - 2 * x);
}

function buildToasterWipe(snapshot: HTMLCanvasElement): Wipe {
  const aspect = window.innerWidth / window.innerHeight;
  const scene = new Scene();

  const H = 6;
  const fov = 50;
  const dist = H / 2 / Math.tan(((fov * Math.PI) / 180) / 2);
  const camera = new PerspectiveCamera(fov, aspect, 0.1, 100);
  camera.position.set(0, 0, dist);
  camera.lookAt(0, 0, 0);
  const W = H * aspect;

  const base = new CanvasTexture(snapshot);
  base.colorSpace = SRGBColorSpace;

  const tileW = W / TILE_COLS;
  const tileH = H / TILE_ROWS;

  interface Tile {
    mesh: Mesh;
    mat: MeshBasicMaterial;
    geo: PlaneGeometry;
    delay: number;
    baseX: number;
    baseY: number;
    spin: number;
  }
  const tiles: Tile[] = [];

  for (let r = 0; r < TILE_ROWS; r++) {
    for (let c = 0; c < TILE_COLS; c++) {
      const geo = new PlaneGeometry(tileW * 0.985, tileH * 0.985);
      // Remap this tile's UVs to its sub-region of the shared snapshot texture.
      const uv = geo.getAttribute('uv') as BufferAttribute;
      const u0 = c / TILE_COLS;
      const u1 = (c + 1) / TILE_COLS;
      const v0 = 1 - (r + 1) / TILE_ROWS;
      const v1 = 1 - r / TILE_ROWS;
      uv.setXY(0, u0, v1);
      uv.setXY(1, u1, v1);
      uv.setXY(2, u0, v0);
      uv.setXY(3, u1, v0);
      uv.needsUpdate = true;

      const mat = new MeshBasicMaterial({ map: base, side: DoubleSide, transparent: true });
      const mesh = new Mesh(geo, mat);
      const baseX = -W / 2 + tileW / 2 + c * tileW;
      const baseY = H / 2 - tileH / 2 - r * tileH;
      mesh.position.set(baseX, baseY, 0);
      scene.add(mesh);

      const delay = ((c + r) / (TILE_COLS + TILE_ROWS - 2)) * WIPE_SPREAD;
      tiles.push({ mesh, mat, geo, delay, baseX, baseY, spin: (r + c) % 2 === 0 ? 1 : -1 });
    }
  }

  // A brief cyan "switcher" flash, the way an old DVE punches the cut.
  const flashMat = new MeshBasicMaterial({
    color: new Color(0x9fe6ff),
    transparent: true,
    opacity: 0,
    depthTest: false,
    toneMapped: false,
  });
  const flash = new Mesh(new PlaneGeometry(W * 1.3, H * 1.3), flashMat);
  flash.position.set(0, 0, dist * 0.5);
  scene.add(flash);

  function step(elapsed: number): boolean {
    flashMat.opacity = Math.max(0, 0.55 - elapsed * 2.4);
    let active = false;
    for (const t of tiles) {
      const lt = (elapsed - t.delay) / WIPE_DUR;
      if (lt < 0) {
        active = true;
        continue;
      }
      if (lt >= 1) {
        t.mesh.visible = false;
        continue;
      }
      active = true;
      const e = smoothstep(lt);
      t.mesh.rotation.y = e * Math.PI * 1.15 * t.spin; // the flip
      t.mesh.rotation.x = e * 0.45; // a little tumble
      t.mesh.position.z = e * 3.4; // fly toward the viewer
      t.mesh.position.x = t.baseX * (1 + e * 0.4); // spread apart
      t.mesh.position.y = t.baseY * (1 + e * 0.22);
      const s = 1 - e * 0.25;
      t.mesh.scale.set(s, s, s);
      t.mat.opacity = lt > 0.7 ? 1 - (lt - 0.7) / 0.3 : 1;
    }
    return active && elapsed < WIPE_SPREAD + WIPE_DUR + 0.2;
  }

  function dispose(): void {
    base.dispose();
    flashMat.dispose();
    flash.geometry.dispose();
    for (const t of tiles) {
      t.geo.dispose();
      t.mat.dispose();
    }
  }

  return { scene, camera, step, dispose };
}

// ---------------------------------------------------------------------------
// The sculpture: the hexagonal architecture as a museum piece.
// ---------------------------------------------------------------------------
type SatKind = 'deck' | 'keys' | 'screen' | 'net' | 'config' | 'tray';

interface Sculpture {
  scene: Scene;
  camera: PerspectiveCamera;
  controls: OrbitControls;
  update(time: number): void;
  dispose(): void;
}

interface Satellite {
  body: Group;
  line: Line;
  anchor: Vector3;
  basePos: Vector3;
  phase: number;
}

function buildSatelliteBody(kind: SatKind): Group {
  const g = new Group();
  switch (kind) {
    case 'deck': {
      const shell = new Mesh(new BoxGeometry(1.1, 0.4, 0.7), new MeshPhongMaterial({ color: PANEL }));
      g.add(shell);
      for (let i = 0; i < 4; i++) {
        const btn = new Mesh(
          new BoxGeometry(0.12, 0.06, 0.12),
          new MeshPhongMaterial({ color: i === 0 ? AMBER : MINT, emissive: new Color(0x123322) }),
        );
        btn.position.set(-0.36 + i * 0.24, 0.23, 0.1);
        g.add(btn);
      }
      break;
    }
    case 'keys': {
      const slab = new Mesh(new BoxGeometry(1.0, 0.16, 0.7), new MeshPhongMaterial({ color: 0x10141b }));
      g.add(slab);
      for (let r = 0; r < 3; r++) {
        for (let c = 0; c < 5; c++) {
          const key = new Mesh(new BoxGeometry(0.12, 0.06, 0.12), new MeshPhongMaterial({ color: 0x2c333f }));
          key.position.set(-0.32 + c * 0.16, 0.11, -0.18 + r * 0.16);
          g.add(key);
        }
      }
      break;
    }
    case 'screen': {
      const bezel = new Mesh(new BoxGeometry(1.1, 0.78, 0.08), new MeshPhongMaterial({ color: 0x0a0c10 }));
      g.add(bezel);
      const face = new Mesh(
        new PlaneGeometry(0.98, 0.64),
        new MeshPhongMaterial({ color: SLATE, emissive: new Color(0x113355) }),
      );
      face.position.z = 0.05;
      g.add(face);
      break;
    }
    case 'net': {
      const globe = new Mesh(
        new CylinderGeometry(0.45, 0.45, 0.45, 18),
        new MeshPhongMaterial({ color: CYAN, transparent: true, opacity: 0.4, shininess: 90 }),
      );
      g.add(globe);
      const ring = new Mesh(new TorusGeometry(0.5, 0.03, 8, 28), new MeshPhongMaterial({ color: MINT }));
      ring.rotation.x = Math.PI / 2.3;
      g.add(ring);
      break;
    }
    case 'config': {
      for (let i = 0; i < 3; i++) {
        const sheet = new Mesh(
          new BoxGeometry(0.7, 0.04, 0.9),
          new MeshPhongMaterial({ color: i === 1 ? 0x223042 : PANEL }),
        );
        sheet.position.y = i * 0.1;
        sheet.rotation.y = i * 0.15;
        g.add(sheet);
      }
      break;
    }
    case 'tray': {
      const disc = new Mesh(
        new CylinderGeometry(0.4, 0.4, 0.12, 24),
        new MeshPhongMaterial({ color: GREEN, emissive: new Color(0x0e5a2c) }),
      );
      disc.rotation.x = Math.PI / 2;
      g.add(disc);
      break;
    }
  }
  return g;
}

function buildSculpture(renderer: WebGLRenderer): Sculpture {
  const scene = new Scene();
  scene.background = new Color(BG);

  const fov = 45;
  const aspect = window.innerWidth / window.innerHeight;
  const camera = new PerspectiveCamera(fov, aspect, 0.1, 80);
  // Distance that fits the sculpture (incl. labels) in both axes — so it doesn't
  // overflow narrow portrait phones, where the limiting factor is width.
  const t = Math.tan((fov * Math.PI) / 180 / 2);
  const dist = Math.min(34, Math.max(2.8 / t, 5.8 / (t * aspect)) * 1.08);
  camera.position.set(0, 1.5, dist);

  scene.add(new AmbientLight(0x2a3a52, 0.4));
  const rimA = new DirectionalLight(0x9fe6c0, 1.4);
  rimA.position.set(-6, 4, -3);
  scene.add(rimA);
  const rimB = new DirectionalLight(CYAN, 0.8);
  rimB.position.set(6, -2, -4);
  scene.add(rimB);
  const fill = new DirectionalLight(0xbfd4ff, 0.4);
  fill.position.set(0, 3, 8);
  scene.add(fill);

  const sculpture = new Group();
  scene.add(sculpture);

  const hexRadius = 1.7;
  const core = new Mesh(
    new CylinderGeometry(hexRadius, hexRadius, 1.1, 6),
    new MeshPhongMaterial({
      color: GREEN,
      transparent: true,
      opacity: 0.32,
      shininess: 80,
      specular: new Color(0xd6ffe9),
    }),
  );
  sculpture.add(core);
  const edges = new LineSegments(
    new EdgesGeometry(core.geometry),
    new LineBasicMaterial({ color: MINT, transparent: true, opacity: 0.85 }),
  );
  core.add(edges);

  const coreLabel = makeLabelSprite('hyperdeck-adapter', { color: '#a6f4c5' });
  coreLabel.position.set(0, 1.2, 0);
  sculpture.add(coreLabel);

  const satDefs: Array<{ kind: SatKind; label: string; port: string; angle: number }> = [
    { kind: 'deck', label: 'HyperDeck TCP :9993', port: 'driving · transport', angle: 30 },
    { kind: 'keys', label: 'Injector', port: 'driven · keystrokes', angle: 90 },
    { kind: 'screen', label: 'UI Automation', port: 'driven · controller', angle: 150 },
    { kind: 'net', label: 'VLC HTTP', port: 'driven · controller', angle: 210 },
    { kind: 'config', label: 'YAML profiles', port: 'driven · ProfileStore', angle: 270 },
    { kind: 'tray', label: 'System tray', port: 'driven · status', angle: 330 },
  ];

  const lineMat = new LineBasicMaterial({ color: MINT, transparent: true, opacity: 0.5 });

  const satellites: Satellite[] = satDefs.map((def, idx) => {
    const a = (def.angle * Math.PI) / 180;
    const dir = new Vector3(Math.cos(a), 0, Math.sin(a));
    const anchor = dir.clone().multiplyScalar(hexRadius * 0.92);
    const basePos = dir.clone().multiplyScalar(4.0);
    basePos.y = idx % 2 === 0 ? 0.6 : -0.55;

    const pivot = new Group();
    sculpture.add(pivot);

    const body = buildSatelliteBody(def.kind);
    body.position.copy(basePos);
    body.lookAt(0, basePos.y, 0);
    pivot.add(body);

    const label = makeLabelSprite(def.label);
    label.position.set(basePos.x, basePos.y + 1.0, basePos.z);
    pivot.add(label);

    const portLabel = makeLabelSprite(def.port, { small: true, color: '#2bd4ff' });
    const portPos = dir.clone().multiplyScalar(hexRadius + 0.36);
    portLabel.position.set(portPos.x, anchor.y + 0.3, portPos.z);
    pivot.add(portLabel);

    const lineGeo = new BufferGeometry().setFromPoints([anchor, basePos]);
    const line = new Line(lineGeo, lineMat);
    pivot.add(line);

    return { body, line, anchor, basePos, phase: idx * 1.4 };
  });

  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.dampingFactor = 0.06;
  controls.enablePan = false;
  controls.minDistance = 4;
  // Must exceed the fitted start distance, or OrbitControls clamps the camera
  // back in and the sculpture overflows narrow (portrait) viewports.
  controls.maxDistance = Math.max(20, dist + 6);
  controls.autoRotate = true;
  controls.autoRotateSpeed = 0.6;
  controls.target.set(0, 0, 0);

  function update(time: number): void {
    sculpture.position.y = Math.sin(time * 0.5) * 0.18;
    core.rotation.y = time * 0.12;

    for (const sat of satellites) {
      const bobY = sat.basePos.y + Math.sin(time * 0.8 + sat.phase) * 0.16;
      sat.body.position.y = bobY;
      const posAttr = sat.line.geometry.getAttribute('position') as BufferAttribute;
      posAttr.setXYZ(0, sat.anchor.x, sat.anchor.y, sat.anchor.z);
      posAttr.setXYZ(1, sat.basePos.x, bobY, sat.basePos.z);
      posAttr.needsUpdate = true;
    }
    controls.update();
  }

  function dispose(): void {
    controls.dispose();
    scene.traverse((obj) => {
      const mesh = obj as Mesh;
      if (mesh.geometry) mesh.geometry.dispose();
      const mat = mesh.material;
      if (Array.isArray(mat)) mat.forEach((mm) => mm.dispose());
      else if (mat) mat.dispose();
    });
  }

  return { scene, camera, controls, update, dispose };
}

// ---------------------------------------------------------------------------
// Orchestration.
// ---------------------------------------------------------------------------
function downscale(src: HTMLCanvasElement, maxW: number): HTMLCanvasElement {
  if (src.width <= maxW) return src;
  const scale = maxW / src.width;
  const out = document.createElement('canvas');
  out.width = Math.round(src.width * scale);
  out.height = Math.round(src.height * scale);
  const ctx = out.getContext('2d')!;
  ctx.drawImage(src, 0, 0, out.width, out.height);
  return out;
}

export async function runEasterEgg(onDone: () => void): Promise<EggController> {
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const vw = window.innerWidth;
  const vh = window.innerHeight;

  let snapshot: HTMLCanvasElement | null = null;
  if (!reducedMotion) {
    try {
      const shot = await html2canvas(document.body, {
        x: window.scrollX,
        y: window.scrollY,
        width: vw,
        height: vh,
        scale: 1,
        backgroundColor: '#0a0c10',
        logging: false,
      });
      snapshot = downscale(shot, 1000);
    } catch {
      snapshot = null;
    }
  }

  const canvas = document.createElement('canvas');
  canvas.id = 'egg-canvas';
  document.body.appendChild(canvas);
  document.body.classList.add('egg-active');

  const renderer = new WebGLRenderer({ canvas, antialias: true });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.setSize(vw, vh);

  const hint = document.createElement('p');
  hint.id = 'egg-hint';
  hint.textContent = 'the hexagonal architecture of hyperdeck-adapter — drag to orbit, Esc to return';
  document.body.appendChild(hint);

  const page = document.getElementById('page');
  page?.classList.add('draining');

  const wipe = snapshot ? buildToasterWipe(snapshot) : null;
  const sculpture = buildSculpture(renderer);
  if (reducedMotion) sculpture.controls.autoRotate = false;

  let phase: 'wipe' | 'sculpture' = wipe ? 'wipe' : 'sculpture';
  if (phase === 'sculpture') hint.classList.add('visible');

  let rafId = 0;
  let disposed = false;
  const start = performance.now();

  function frame(now: number): void {
    if (disposed) return;
    rafId = requestAnimationFrame(frame);
    const elapsed = (now - start) / 1000;

    sculpture.update(elapsed);
    renderer.autoClear = true;
    renderer.render(sculpture.scene, sculpture.camera);

    if (phase === 'wipe' && wipe) {
      const still = wipe.step(elapsed);
      renderer.autoClear = false;
      renderer.render(wipe.scene, wipe.camera);
      renderer.autoClear = true;
      if (!still) {
        phase = 'sculpture';
        hint.classList.add('visible');
      }
    }
  }
  rafId = requestAnimationFrame(frame);

  function onVisibility(): void {
    if (disposed) return;
    if (document.hidden) {
      cancelAnimationFrame(rafId);
    } else {
      rafId = requestAnimationFrame(frame);
    }
  }
  document.addEventListener('visibilitychange', onVisibility);

  function onResize(): void {
    if (disposed) return;
    renderer.setSize(window.innerWidth, window.innerHeight);
    sculpture.camera.aspect = window.innerWidth / window.innerHeight;
    sculpture.camera.updateProjectionMatrix();
  }
  window.addEventListener('resize', onResize);

  function dismiss(): void {
    if (disposed) return;
    disposed = true;
    cancelAnimationFrame(rafId);
    document.removeEventListener('visibilitychange', onVisibility);
    window.removeEventListener('resize', onResize);

    canvas.style.transition = 'opacity 0.45s ease';
    canvas.style.opacity = '0';
    hint.classList.remove('visible');
    page?.classList.remove('draining');
    window.setTimeout(() => {
      sculpture.dispose();
      wipe?.dispose();
      renderer.dispose();
      canvas.remove();
      hint.remove();
      document.body.classList.remove('egg-active');
      onDone();
    }, 470);
  }

  return { dismiss };
}
