// The hero diorama: a dark broadcast control room. A 50-year-old technical
// director stands behind three seated operators (media, switcher, camera), all
// facing a glowing multiviewer wall and a window that looks out over a darkened
// theater — rows of audience watching a lit stage. Built from primitives.

import {
  AmbientLight,
  BoxGeometry,
  CanvasTexture,
  Color,
  CylinderGeometry,
  DirectionalLight,
  Fog,
  Group,
  HemisphereLight,
  InstancedMesh,
  Matrix4,
  Mesh,
  MeshBasicMaterial,
  MeshStandardMaterial,
  Object3D,
  PerspectiveCamera,
  PlaneGeometry,
  PointLight,
  Scene,
  SphereGeometry,
  SpotLight,
  SRGBColorSpace,
  WebGLRenderer,
  DoubleSide,
} from 'three';

const PALETTE = {
  bg: 0x07090d,
  floor: 0x10141b,
  wall: 0x161c25,
  deskTop: 0x0d1117,
  deskBody: 0x191f29,
  metal: 0x222a35,
  skinA: 0xc99a72,
  skinB: 0xd8b48f,
  skinC: 0xb6855e,
  skinTD: 0xcaa385,
  hairDark: 0x23272f,
  hairGrey: 0x9aa0a8,
  shirtMedia: 0x2f6fb0, // cyan-blue
  shirtSwitch: 0x2fa46a, // green
  shirtCamera: 0x8a5fb0, // violet
  vestTD: 0x2a2f3a,
  headset: 0x111418,
  seat: 0x1a1f28,
  audience: 0x1a212b,
  stageWarm: 0xffce8a,
};

const screens: Array<{ mat: MeshBasicMaterial; base: Color; phase: number; speed: number }> = [];
const tallies: Array<{ mat: MeshBasicMaterial; phase: number }> = [];

function surface(color: number, opts?: { rough?: number; metal?: number }): MeshStandardMaterial {
  return new MeshStandardMaterial({
    color,
    roughness: opts?.rough ?? 0.85,
    metalness: opts?.metal ?? 0.1,
    flatShading: true,
  });
}

// A glowing screen face. Registered for per-frame flicker.
function screenFace(w: number, h: number, color: number, intensity = 1): Mesh {
  const base = new Color(color).multiplyScalar(intensity);
  const mat = new MeshBasicMaterial({ color: base.clone(), toneMapped: false });
  const mesh = new Mesh(new PlaneGeometry(w, h), mat);
  screens.push({ mat, base, phase: Math.random() * Math.PI * 2, speed: 0.4 + Math.random() * 0.9 });
  return mesh;
}

function tally(color: number, r = 0.03): Mesh {
  const mat = new MeshBasicMaterial({ color: new Color(color), toneMapped: false });
  const mesh = new Mesh(new SphereGeometry(r, 8, 8), mat);
  tallies.push({ mat, phase: Math.random() * Math.PI * 2 });
  return mesh;
}

// A classic Academy-leader film countdown drawn to a canvas texture: a rotating
// sweep, crosshair, concentric rings, and a big number cycling 10 -> 1, looping.
// The returned texture is shared by the stage screen and a control-room monitor.
interface Countdown {
  texture: CanvasTexture;
  update(elapsed: number): void;
}

function makeCountdown(): Countdown {
  const w = 640;
  const h = 480;
  const canvas = document.createElement('canvas');
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext('2d')!;
  const texture = new CanvasTexture(canvas);
  texture.colorSpace = SRGBColorSpace;
  const cx = w / 2;
  const cy = h / 2;
  const R = Math.min(cx, cy);

  function update(elapsed: number): void {
    const frac = elapsed % 1; // position within the current second
    const number = 10 - (Math.floor(elapsed) % 10); // 10..1, looping

    ctx.fillStyle = '#8b9099';
    ctx.fillRect(0, 0, w, h);

    // Sweeping wedge, clockwise from the top — the leader's rotating hand.
    ctx.fillStyle = '#6b7079';
    ctx.beginPath();
    ctx.moveTo(cx, cy);
    const a0 = -Math.PI / 2;
    ctx.arc(cx, cy, w, a0, a0 + frac * Math.PI * 2);
    ctx.closePath();
    ctx.fill();

    ctx.strokeStyle = '#191c22';
    ctx.lineWidth = 6;
    ctx.beginPath();
    ctx.arc(cx, cy, R * 0.92, 0, Math.PI * 2);
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(cx, cy, R * 0.6, 0, Math.PI * 2);
    ctx.stroke();

    // Full-frame crosshair.
    ctx.lineWidth = 4;
    ctx.beginPath();
    ctx.moveTo(cx, 0);
    ctx.lineTo(cx, h);
    ctx.moveTo(0, cy);
    ctx.lineTo(w, cy);
    ctx.stroke();

    ctx.fillStyle = '#0e1116';
    ctx.font = `bold ${Math.round(R * 0.85)}px Georgia, "Times New Roman", serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(String(number), cx, cy + R * 0.04);

    texture.needsUpdate = true;
  }

  update(0);
  return { texture, update };
}

// ---------------------------------------------------------------------------
// Figures.
// ---------------------------------------------------------------------------
interface Figure {
  group: Group;
  head: Group;
  torso: Mesh;
  phase: number;
}

function addHeadset(head: Group): void {
  const band = new Mesh(new BoxGeometry(0.04, 0.26, 0.04), surface(PALETTE.headset, { rough: 0.6 }));
  band.position.set(0.16, 0.02, 0);
  band.rotation.z = 0.25;
  head.add(band);
  const cup = new Mesh(new BoxGeometry(0.07, 0.1, 0.1), surface(PALETTE.headset, { rough: 0.6 }));
  cup.position.set(0.18, -0.05, 0);
  head.add(cup);
  // Mic boom.
  const boom = new Mesh(new BoxGeometry(0.18, 0.02, 0.02), surface(PALETTE.headset, { rough: 0.6 }));
  boom.position.set(0.12, -0.12, 0.12);
  boom.rotation.z = -0.5;
  boom.rotation.y = -0.5;
  head.add(boom);
}

// A seated operator at the desk, facing -Z.
function buildOperator(skin: number, shirt: number, hair: number): Figure {
  const group = new Group();

  const seat = new Mesh(new BoxGeometry(0.5, 0.1, 0.5), surface(PALETTE.seat));
  seat.position.y = 0.5;
  group.add(seat);
  const seatBack = new Mesh(new BoxGeometry(0.5, 0.55, 0.08), surface(PALETTE.seat));
  seatBack.position.set(0, 0.78, 0.24);
  group.add(seatBack);

  // Thighs (seated): horizontal, forward toward the desk.
  const thighs = new Mesh(new BoxGeometry(0.42, 0.16, 0.5), surface(0x20252e));
  thighs.position.set(0, 0.6, -0.18);
  group.add(thighs);

  // Torso, leaning slightly forward over the desk.
  const torso = new Mesh(new BoxGeometry(0.46, 0.62, 0.3), surface(shirt, { rough: 0.95 }));
  torso.position.set(0, 0.98, 0.0);
  torso.rotation.x = -0.18;
  group.add(torso);

  // Shoulders.
  const shoulders = new Mesh(new BoxGeometry(0.56, 0.16, 0.3), surface(shirt, { rough: 0.95 }));
  shoulders.position.set(0, 1.22, -0.04);
  group.add(shoulders);

  // Arms reaching forward to the desk surface.
  for (const side of [-1, 1]) {
    const upper = new Mesh(new BoxGeometry(0.13, 0.13, 0.34), surface(shirt, { rough: 0.95 }));
    upper.position.set(0.22 * side, 1.12, -0.22);
    upper.rotation.x = -0.5;
    group.add(upper);
    const fore = new Mesh(new BoxGeometry(0.11, 0.11, 0.34), surface(skin));
    fore.position.set(0.22 * side, 1.0, -0.46);
    fore.rotation.x = -1.0;
    group.add(fore);
  }

  // Head.
  const head = new Group();
  head.position.set(0, 1.4, -0.06);
  group.add(head);
  const skull = new Mesh(new SphereGeometry(0.17, 12, 10), surface(skin));
  skull.scale.set(1, 1.12, 1.05);
  head.add(skull);
  const hairCap = new Mesh(new SphereGeometry(0.18, 12, 10), surface(hair, { rough: 1 }));
  hairCap.scale.set(1.02, 0.7, 1.05);
  hairCap.position.y = 0.07;
  head.add(hairCap);
  addHeadset(head);

  return { group, head, torso, phase: Math.random() * Math.PI * 2 };
}

// The standing technical director, facing -Z. Greying hair; one hand to headset.
function buildTD(): Figure {
  const group = new Group();
  const skin = PALETTE.skinTD;

  // Legs.
  for (const side of [-1, 1]) {
    const leg = new Mesh(new BoxGeometry(0.17, 0.92, 0.2), surface(0x1d222b));
    leg.position.set(0.12 * side, 0.46, 0);
    group.add(leg);
    const shoe = new Mesh(new BoxGeometry(0.18, 0.1, 0.32), surface(0x0c0e12));
    shoe.position.set(0.12 * side, 0.05, -0.06);
    group.add(shoe);
  }

  // Torso with a dark vest over a lighter shirt.
  const torso = new Mesh(new BoxGeometry(0.5, 0.72, 0.3), surface(PALETTE.vestTD, { rough: 0.9 }));
  torso.position.set(0, 1.28, 0);
  group.add(torso);
  const collar = new Mesh(new BoxGeometry(0.2, 0.18, 0.31), surface(0x39414e));
  collar.position.set(0, 1.56, 0.01);
  group.add(collar);
  const shoulders = new Mesh(new BoxGeometry(0.62, 0.18, 0.3), surface(PALETTE.vestTD, { rough: 0.9 }));
  shoulders.position.set(0, 1.62, 0);
  group.add(shoulders);

  // Left arm at side; right arm raised, hand to the headset (calling the show).
  const leftUpper = new Mesh(new BoxGeometry(0.14, 0.5, 0.16), surface(PALETTE.vestTD, { rough: 0.9 }));
  leftUpper.position.set(-0.33, 1.34, 0);
  group.add(leftUpper);
  const leftFore = new Mesh(new BoxGeometry(0.12, 0.42, 0.14), surface(skin));
  leftFore.position.set(-0.33, 0.96, 0.02);
  group.add(leftFore);

  const rightUpper = new Mesh(new BoxGeometry(0.14, 0.42, 0.16), surface(PALETTE.vestTD, { rough: 0.9 }));
  rightUpper.position.set(0.33, 1.4, 0.02);
  rightUpper.rotation.z = 0.5;
  group.add(rightUpper);
  const rightFore = new Mesh(new BoxGeometry(0.12, 0.34, 0.14), surface(skin));
  rightFore.position.set(0.45, 1.66, 0.08);
  rightFore.rotation.z = 1.1;
  group.add(rightFore);

  // Head, greying.
  const head = new Group();
  head.position.set(0, 1.84, 0);
  group.add(head);
  const skull = new Mesh(new SphereGeometry(0.18, 14, 12), surface(skin));
  skull.scale.set(1, 1.14, 1.05);
  head.add(skull);
  const hairCap = new Mesh(new SphereGeometry(0.19, 14, 12), surface(PALETTE.hairGrey, { rough: 1 }));
  hairCap.scale.set(1.02, 0.55, 1.05);
  hairCap.position.y = 0.08;
  head.add(hairCap);
  addHeadset(head);

  return { group, head, torso, phase: 0 };
}

// ---------------------------------------------------------------------------
// Control room furniture.
// ---------------------------------------------------------------------------
function buildDesk(countdown: CanvasTexture): Group {
  const g = new Group();
  const top = new Mesh(new BoxGeometry(5.4, 0.1, 1.3), surface(PALETTE.deskTop, { rough: 0.5, metal: 0.3 }));
  top.position.set(0, 0.95, 2.2);
  g.add(top);
  const front = new Mesh(new BoxGeometry(5.4, 0.95, 0.1), surface(PALETTE.deskBody));
  front.position.set(0, 0.48, 2.8);
  g.add(front);

  // A back riser holding small desk monitors per operator.
  const riser = new Mesh(new BoxGeometry(5.4, 0.06, 0.4), surface(PALETTE.metal, { metal: 0.5 }));
  riser.position.set(0, 1.0, 1.7);
  g.add(riser);

  const deskScreenColors = [0x2bd4ff, 0x39ff9e, 0xffb347];
  [-1.7, 0, 1.7].forEach((x, i) => {
    // Two small angled monitors per station. The switcher's left monitor mirrors
    // the film-leader countdown.
    for (const dx of [-0.42, 0.42]) {
      const bezel = new Mesh(new BoxGeometry(0.7, 0.46, 0.04), surface(0x0a0c10, { metal: 0.4 }));
      bezel.position.set(x + dx, 1.3, 1.62);
      bezel.rotation.x = -0.12;
      g.add(bezel);
      let face: Mesh;
      if (i === 1 && dx === -0.42) {
        face = new Mesh(
          new PlaneGeometry(0.62, 0.38),
          new MeshBasicMaterial({ map: countdown, toneMapped: false }),
        );
      } else {
        face = screenFace(0.62, 0.38, deskScreenColors[i], 0.7);
      }
      face.position.set(x + dx, 1.3, 1.643);
      face.rotation.x = -0.12;
      g.add(face);
    }
    // A T-bar / panel hint on the desk in front of each operator.
    const panel = new Mesh(new BoxGeometry(0.8, 0.05, 0.4), surface(0x12161d, { metal: 0.4 }));
    panel.position.set(x, 1.01, 2.2);
    g.add(panel);
    const t = tally(i === 1 ? 0xff3b30 : 0x39ff9e);
    t.position.set(x, 1.05, 2.05);
    g.add(t);
  });

  return g;
}

// The multiviewer wall above the window: a grid of glowing monitors facing the room.
// One monitor mirrors the stage's film-leader countdown.
function buildMultiviewer(countdown: CanvasTexture): Group {
  const g = new Group();
  const cols = 8;
  const rows = 2;
  const countdownCol = 3; // which monitor in the top row shows the leader
  const sw = 0.78;
  const sh = 0.5;
  const gap = 0.06;
  const centerY = 4.25; // mounted high, above the window
  const totalW = cols * sw + (cols - 1) * gap;
  const totalH = rows * sh + (rows - 1) * gap;
  const palette = [0x1f6feb, 0x2bd4ff, 0x39ff9e, 0xffb347, 0xb084ff, 0x4cc9f0];

  const backing = new Mesh(
    new BoxGeometry(totalW + 0.3, totalH + 0.3, 0.12),
    surface(0x05070a, { metal: 0.3 }),
  );
  backing.position.set(0, centerY, -0.62);
  g.add(backing);

  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const x = -totalW / 2 + sw / 2 + c * (sw + gap);
      const y = centerY + totalH / 2 - sh / 2 - r * (sh + gap);
      let face: Mesh;
      if (r === 0 && c === countdownCol) {
        face = new Mesh(
          new PlaneGeometry(sw - 0.06, sh - 0.06),
          new MeshBasicMaterial({ map: countdown, toneMapped: false }),
        );
      } else {
        const color = palette[(r * cols + c) % palette.length];
        face = screenFace(sw - 0.06, sh - 0.06, color, 0.55);
      }
      face.position.set(x, y, -0.55);
      g.add(face);
    }
  }
  return g;
}

// ---------------------------------------------------------------------------
// The theater beyond the control-room window.
// ---------------------------------------------------------------------------
function buildTheater(scene: Scene, countdown: CanvasTexture): void {
  const g = new Group();
  scene.add(g);

  // House floor, sloping down toward the stage.
  const floor = new Mesh(new PlaneGeometry(34, 26), surface(0x080a0e, { rough: 1 }));
  floor.rotation.x = -Math.PI / 2 + 0.14;
  floor.position.set(0, -1.0, -9);
  g.add(floor);

  // Audience: instanced seated silhouettes in curved rows facing the stage.
  const rowsN = 9;
  const colsN = 18;
  const heads = new InstancedMesh(
    new SphereGeometry(0.16, 8, 6),
    surface(PALETTE.audience, { rough: 1 }),
    rowsN * colsN,
  );
  const shoulders = new InstancedMesh(
    new BoxGeometry(0.4, 0.34, 0.26),
    surface(PALETTE.audience, { rough: 1 }),
    rowsN * colsN,
  );
  const m = new Matrix4();
  const dummy = new Object3D();
  let idx = 0;
  for (let r = 0; r < rowsN; r++) {
    const z = -3.4 - r * 1.0;
    const y = -1.2 - r * 0.2; // descend toward stage
    for (let c = 0; c < colsN; c++) {
      const x = (c - (colsN - 1) / 2) * 0.62 + (Math.random() - 0.5) * 0.12;
      const jy = (Math.random() - 0.5) * 0.06;
      dummy.position.set(x, y + 0.7 + jy, z);
      dummy.rotation.set(0, 0, 0);
      dummy.updateMatrix();
      heads.setMatrixAt(idx, dummy.matrix);
      dummy.position.set(x, y + 0.42 + jy, z);
      dummy.updateMatrix();
      m.copy(dummy.matrix);
      shoulders.setMatrixAt(idx, m);
      idx++;
    }
  }
  heads.instanceMatrix.needsUpdate = true;
  shoulders.instanceMatrix.needsUpdate = true;
  g.add(heads);
  g.add(shoulders);

  // Proscenium arch framing the stage.
  const archMat = surface(0x0c0f15, { rough: 1 });
  const archTop = new Mesh(new BoxGeometry(13, 1.4, 1), archMat);
  archTop.position.set(0, 4.4, -12);
  g.add(archTop);
  for (const side of [-1, 1]) {
    const leg = new Mesh(new BoxGeometry(1.4, 10, 1), archMat);
    leg.position.set(6 * side, -0.2, -12);
    g.add(leg);
  }

  // Stage: a raised platform, warmly lit, with a backdrop and a speaker figure.
  const stage = new Mesh(new BoxGeometry(11, 0.7, 4), surface(0x1a140b, { rough: 0.9 }));
  stage.position.set(0, -1.2, -14);
  g.add(stage);
  const backdrop = new Mesh(new PlaneGeometry(12, 8.5), surface(0x0c0d10, { rough: 1 }));
  backdrop.position.set(0, 2.2, -15.6);
  g.add(backdrop);

  // The screen behind the stage: a projection of the looping film leader.
  const projScreen = new Mesh(
    new PlaneGeometry(8, 6),
    new MeshBasicMaterial({ map: countdown, toneMapped: false }),
  );
  projScreen.position.set(0, 2.4, -15.45);
  g.add(projScreen);

  // A lone presenter on the stage, lit.
  const presenter = new Group();
  presenter.position.set(0, -0.85, -14);
  const body = new Mesh(new CylinderGeometry(0.22, 0.3, 1.1, 8), surface(0x3a3326));
  body.position.y = 0.55;
  presenter.add(body);
  const phead = new Mesh(new SphereGeometry(0.18, 10, 8), surface(PALETTE.skinB));
  phead.position.y = 1.25;
  presenter.add(phead);
  g.add(presenter);

  // A warm wash that makes the stage glow against the dark house.
  const glow = new Mesh(
    new PlaneGeometry(11, 3.2),
    new MeshBasicMaterial({ color: new Color(PALETTE.stageWarm).multiplyScalar(0.45), transparent: true, opacity: 0.55, side: DoubleSide, toneMapped: false }),
  );
  glow.position.set(0, -0.2, -15.0); // washes the stage deck, below the screen
  g.add(glow);
}

// ---------------------------------------------------------------------------
// Assembly + run loop.
// ---------------------------------------------------------------------------
export function startControlRoom(hero: HTMLElement): void {
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  const canvas = document.createElement('canvas');
  hero.appendChild(canvas);

  const renderer = new WebGLRenderer({ canvas, antialias: true, alpha: false });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.setSize(hero.clientWidth, hero.clientHeight);

  const scene = new Scene();
  scene.background = new Color(PALETTE.bg);
  scene.fog = new Fog(PALETTE.bg, 16, 50);

  const camera = new PerspectiveCamera(50, hero.clientWidth / hero.clientHeight, 0.1, 80);
  camera.position.set(3.9, 4.4, 9.9);
  camera.lookAt(-0.2, 1.5, -6);

  // --- Lighting: dim, screen-lit, with warm accents on the TD and stage. ---
  scene.add(new AmbientLight(0x223044, 0.5));
  scene.add(new HemisphereLight(0x2a3a52, 0x05070a, 0.5));

  const coolKey = new DirectionalLight(0x88a6cc, 0.5);
  coolKey.position.set(-4, 6, 4);
  scene.add(coolKey);

  // Cyan glow from the desk monitors onto the operators.
  const deskGlow = new PointLight(0x2bd4ff, 18, 9, 2);
  deskGlow.position.set(0, 1.5, 2.6);
  scene.add(deskGlow);

  // Blue spill from the multiviewer wall.
  const wallGlow = new PointLight(0x1f6feb, 22, 12, 2);
  wallGlow.position.set(0, 2.9, 0.4);
  scene.add(wallGlow);

  // Warm key on the standing TD.
  const tdSpot = new SpotLight(0xffdca8, 45, 12, 0.7, 0.5, 1.5);
  tdSpot.position.set(3.4, 4.8, 8.4);
  const tdTarget = new Object3D();
  tdTarget.position.set(1.2, 1.4, 4.9);
  scene.add(tdTarget);
  tdSpot.target = tdTarget;
  scene.add(tdSpot);

  // Faint cool wash over the front of the house so the audience reads.
  const houseFill = new PointLight(0x3a5a82, 9, 16, 2);
  houseFill.position.set(0, 2.6, -6);
  scene.add(houseFill);

  // Warm wash on the distant stage.
  const stageSpot = new SpotLight(0xffce8a, 70, 26, 0.5, 0.4, 1.2);
  stageSpot.position.set(0, 8, -9);
  const stageTarget = new Object3D();
  stageTarget.position.set(0, -0.6, -14);
  scene.add(stageTarget);
  stageSpot.target = stageTarget;
  scene.add(stageSpot);

  // --- Room shell (floor, side walls, lintel, sill) framing the window. ---
  const room = new Group();
  scene.add(room);

  const floor = new Mesh(new PlaneGeometry(16, 12), surface(PALETTE.floor, { rough: 1 }));
  floor.rotation.x = -Math.PI / 2;
  floor.position.set(0, 0, 3);
  room.add(floor);

  // A wide window onto the theater: low sill, tall side jambs, and a lintel above
  // (the multiviewer strip mounts on the lintel).
  const sill = new Mesh(new BoxGeometry(9, 0.5, 0.3), surface(PALETTE.wall));
  sill.position.set(0, 0.25, -0.7);
  room.add(sill);
  const lintel = new Mesh(new BoxGeometry(9, 1.8, 0.3), surface(PALETTE.wall));
  lintel.position.set(0, 4.6, -0.7);
  room.add(lintel);
  for (const side of [-1, 1]) {
    const jamb = new Mesh(new BoxGeometry(1.0, 5.5, 0.3), surface(PALETTE.wall));
    jamb.position.set(4.0 * side, 2.75, -0.7);
    room.add(jamb);
    const wall = new Mesh(new BoxGeometry(3, 6, 0.2), surface(PALETTE.wall, { rough: 1 }));
    wall.position.set(6 * side, 3, 3);
    wall.rotation.y = Math.PI / 2;
    room.add(wall);
  }
  const ceiling = new Mesh(new PlaneGeometry(16, 12), surface(0x0b0e13, { rough: 1 }));
  ceiling.rotation.x = Math.PI / 2;
  ceiling.position.set(0, 5.5, 3);
  room.add(ceiling);

  const countdown = makeCountdown();
  room.add(buildDesk(countdown.texture));
  room.add(buildMultiviewer(countdown.texture));

  // Operators: media (cyan), switcher (green), camera (violet).
  const operators: Figure[] = [];
  const opDefs = [
    { x: -1.7, skin: PALETTE.skinA, shirt: PALETTE.shirtMedia, hair: PALETTE.hairDark },
    { x: 0, skin: PALETTE.skinB, shirt: PALETTE.shirtSwitch, hair: PALETTE.hairDark },
    { x: 1.7, skin: PALETTE.skinC, shirt: PALETTE.shirtCamera, hair: 0x4a3526 },
  ];
  for (const d of opDefs) {
    const op = buildOperator(d.skin, d.shirt, d.hair);
    op.group.position.set(d.x, 0, 3.25);
    room.add(op.group);
    operators.push(op);
  }

  // The technical director, standing in the room, overseeing.
  const td = buildTD();
  td.group.position.set(1.2, 0, 4.9);
  room.add(td.group);

  buildTheater(scene, countdown.texture);

  // --- Animation ---
  let rafId = 0;
  let disposed = false;
  const start = performance.now();

  function render(elapsed: number): void {
    // Film-leader countdown (shared by the stage screen and a control-room monitor).
    countdown.update(elapsed);
    // Gentle screen shimmer (slow, low-amplitude — not a strobe).
    for (const s of screens) {
      const f = 0.94 + 0.06 * Math.sin(elapsed * s.speed + s.phase);
      s.mat.color.copy(s.base).multiplyScalar(f);
    }
    // Tally lights: a soft pulse, mostly on (no hard blink).
    for (const t of tallies) {
      const on = 0.72 + 0.28 * Math.sin(elapsed * 1.1 + t.phase);
      t.mat.color.setRGB(on, on * 0.12, on * 0.1);
    }
    // Operators breathe and shift slightly.
    for (const op of operators) {
      op.torso.scale.y = 1 + Math.sin(elapsed * 1.4 + op.phase) * 0.015;
      op.head.rotation.y = Math.sin(elapsed * 0.5 + op.phase) * 0.12;
      op.head.rotation.x = Math.sin(elapsed * 0.4 + op.phase * 1.3) * 0.05;
    }
    // TD: subtle weight shift and a slow head turn as he watches the room/stage.
    td.group.rotation.y = Math.sin(elapsed * 0.3) * 0.05;
    td.head.rotation.y = Math.sin(elapsed * 0.25 + 1) * 0.25;

    // Slow camera drift for parallax.
    const drift = reducedMotion ? 0 : Math.sin(elapsed * 0.12);
    camera.position.x = 3.9 + drift * 0.5;
    camera.position.y = 4.4 + Math.cos(elapsed * 0.1) * 0.12;
    camera.lookAt(-0.2, 1.5, -6);

    renderer.render(scene, camera);
  }

  function frame(now: number): void {
    if (disposed) return;
    rafId = requestAnimationFrame(frame);
    render((now - start) / 1000);
  }

  if (reducedMotion) {
    render(0); // single static frame
  } else {
    rafId = requestAnimationFrame(frame);
  }

  function onResize(): void {
    if (disposed) return;
    const w = hero.clientWidth;
    const h = hero.clientHeight;
    renderer.setSize(w, h);
    camera.aspect = w / h;
    camera.updateProjectionMatrix();
  }
  window.addEventListener('resize', onResize);

  function onVisibility(): void {
    if (disposed || reducedMotion) return;
    if (document.hidden) {
      cancelAnimationFrame(rafId);
    } else {
      rafId = requestAnimationFrame(frame);
    }
  }
  document.addEventListener('visibilitychange', onVisibility);
}
