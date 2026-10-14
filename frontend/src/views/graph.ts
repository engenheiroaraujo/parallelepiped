// 3D graph view, styled as a field of neurons. Drag rotates it (orbit
// controls), the wheel zooms, right-drag pans, clicking a node opens it.
//
// Each note is a neuron: a lumpy cell body glowing from within, in its
// folder's colour, with organic dendrites (see neuron.ts). Links are fibres
// tinted by their source neuron, with faint pulses running towards the
// target. Fog fades distant neurons into the dark background for depth.
//
// Rendering quality: the canvas is rendered above the screen's pixel ratio
// (supersampling) with antialiasing, and a bloom pass makes the cell bodies
// glow. Labels stay under the bloom threshold so they remain crisp.

import ForceGraph3D, { type ConfigOptions, type ForceGraph3DInstance, type LinkObject, type NodeObject } from '3d-force-graph';
import SpriteText from 'three-spritetext';
import {
  AdditiveBlending,
  AmbientLight,
  type BufferGeometry,
  CanvasTexture,
  Color,
  DirectionalLight,
  FogExp2,
  Group,
  Mesh,
  MeshStandardMaterial,
  Sprite,
  SpriteMaterial,
  Vector2,
} from 'three';
import { UnrealBloomPass } from 'three/examples/jsm/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/examples/jsm/postprocessing/OutputPass.js';
import type { GraphData, GraphNodeData } from '../api/api';
import { folderColors, ROOT_COLOR, UNRESOLVED_COLOR } from '../ui/colors';
import { currentTheme, onThemeChange, type Theme } from '../ui/theme';
import { buildNeuron, hash } from './neuron';

type Node = NodeObject & GraphNodeData;
type Link = LinkObject<Node>;

// The default export is not generic over node/link types; narrow it once here.
const Graph3D = ForceGraph3D as unknown as new (element: HTMLElement, config?: ConfigOptions) => ForceGraph3DInstance<Node, Link>;

// Per-theme scene colours. Folder colours come from ui/colors so the graph
// matches the file tree. Bloom only suits the dark theme: on a light
// background it washes the whole scene out.
interface SceneLook {
  background: string;
  label: string;
  dim: string;
  highlight: string;
  dendriteTint: Color;
  bloom: boolean;
}

const SCENE_LOOKS: Record<Theme, SceneLook> = {
  dark: { background: '#1e1e1e', label: '#9a9aa4', dim: '#2c2c32', highlight: '#f4f4f5', dendriteTint: new Color('#2a201c'), bloom: true },
  light: { background: '#ecebe7', label: '#4a4a52', dim: '#d6d5d0', highlight: '#18181b', dendriteTint: new Color('#3a3631'), bloom: false },
};

// Neurons.
// Like Obsidian, a note with more links is bigger: the radius grows with the
// square root of its link count, capped so hubs do not swallow the view.
const SOMA_BASE_RADIUS = 1.2;
const SOMA_DEGREE_RADIUS = 0.9;
const SOMA_MAX_RADIUS = 12;
const SOMA_GLOW = 0.5; // emissive intensity of the cell body
const DENDRITE_SHADE = 0.55; // how far dendrites shade towards the scene's dendriteTint
const CORE_SCALE = 2.4; // inner glow diameter, in soma radii
const CORE_OPACITY = 0.38;

// Fibres between neurons.
const FIBRE_SHADE = 0.45;
const FIBRE_OPACITY = 0.6;
const FIBRE_WIDTH = 0.22;
const FIBRE_CURVATURE = 0.2;
// Pulses run only on the fibres of the hovered neuron: constant pulses on
// every fibre kept the GPU busy redrawing an otherwise still scene.
const PULSES_PER_FIBRE = 2;
const PULSE_SPEED = 0.004;
const PULSE_WIDTH = 0.55;

// Exponential fog: distant neurons fade into the background. The density is
// FOG_SCALE divided by the graph's radius, so a large vault (seen from far
// away) is not swallowed by fog while a small one still shows depth.
const FOG_SCALE = 0.18;
const FOG_DEFAULT_DENSITY = 0.002;

// Labels: dim enough to stay under the bloom threshold (colour per theme).
const LABEL_HEIGHT = 2.4;
// Label texture resolution in px: sharp at normal zoom without the memory of
// very large textures (one per note).
const LABEL_FONT_PX = 72;

// Layout forces (d3 defaults: charge -30, link distance 30).
const CHARGE_BASE = 90;
const CHARGE_PER_RADIUS = 25;
const CHARGE_MAX_DISTANCE = 800;
const LINK_BASE_DISTANCE = 45;
const LINK_RADIUS_FACTOR = 4;
// A weak pull towards the centre. Notes without links feel only repulsion,
// so without it they drift far out and the camera framing shrinks the
// connected cluster to a speck; orphans get a stronger pull.
const GRAVITY = 0.008;
const ORPHAN_GRAVITY = 0.05;

const WARMUP_TICKS = 120;
const COOLDOWN_MS = 3000;
// Delay before the first framing: enough for the graph to apply the data
// and run the warm-up ticks.
const FIRST_FIT_DELAY_MS = 300;

// Render at the screen's own density (capped): antialiasing keeps edges
// smooth, and supersampling multiplied the GPU work by four.
const MAX_PIXEL_RATIO = 2;

// On-demand rendering: the scene is redrawn while the layout settles, the
// camera moves or the user interacts, and paused after this much quiet. Any
// camera animation must finish within it.
const IDLE_PAUSE_MS = 1500;

const BLOOM = { strength: 0.5, radius: 0.45, threshold: 0.6 };

const coreTexture = createGlowTexture();

interface NeuronLook {
  soma: MeshStandardMaterial;
  dendrites: MeshStandardMaterial;
  core: SpriteMaterial;
  geometries: BufferGeometry[];
}

export class GraphView {
  private graph: ForceGraph3DInstance<Node, Link> | null = null;
  private colors = new Map<string, string>();
  private neighbors = new Map<string, Set<string>>();
  private looks = new Map<string, NeuronLook>();
  private nodesById = new Map<string, Node>();
  private hovered: string | null = null;
  private pendingData: GraphData | null = null;
  private needsFit = false;
  private engineRunning = false;
  private paused = false;
  private idleTimer = 0;
  private look: SceneLook = SCENE_LOOKS[currentTheme()];
  private bloomPass: UnrealBloomPass | null = null;

  constructor(
    private readonly container: HTMLElement,
    private readonly onOpen: (node: GraphNodeData) => void,
  ) {
    new ResizeObserver(() => this.resize()).observe(container);
    onThemeChange((theme) => this.setTheme(theme));
  }

  /** Recolours the scene and rebuilds the neurons (labels bake their colour). */
  private setTheme(theme: Theme): void {
    this.look = SCENE_LOOKS[theme];
    const graph = this.graph;
    if (!graph) return;
    const scene = graph.scene();
    scene.background = new Color(this.look.background);
    if (scene.fog instanceof FogExp2) scene.fog.color.set(this.look.background);
    if (this.bloomPass) this.bloomPass.enabled = this.look.bloom;
    for (const look of this.looks.values()) disposeLook(look);
    this.looks.clear();
    graph.nodeThreeObject(graph.nodeThreeObject()).linkColor(graph.linkColor());
    this.wake();
  }

  /** Stores the data; it is drawn when the view becomes visible. */
  setData(data: GraphData): void {
    this.pendingData = data;
    if (this.graph && this.isVisible()) this.draw();
  }

  show(): void {
    if (!this.graph) this.create();
    this.resize();
    this.draw();
  }

  private create(): void {
    const graph = new Graph3D(this.container, {
      controlType: 'orbit',
      rendererConfig: { antialias: true, alpha: false, powerPreference: 'high-performance' },
    })
      .backgroundColor(this.look.background)
      .showNavInfo(false)
      .nodeVal((n) => 1 + n.degree)
      .nodeLabel((n) => n.id)
      .nodeThreeObject((n) => this.nodeObject(n))
      // Fibres: curved tubes tinted by the neuron they leave from.
      .linkCurvature(FIBRE_CURVATURE)
      .linkCurveRotation((l) => hash(`${idOf(l.source)}>${idOf(l.target)}`) * Math.PI * 2)
      .linkColor((l) => this.fibreColor(l))
      .linkOpacity(FIBRE_OPACITY)
      .linkWidth((l) => (this.isLinkHighlighted(l) ? FIBRE_WIDTH * 2.2 : FIBRE_WIDTH))
      .linkResolution(6)
      // Pulses: faint signals running from source to target.
      .linkDirectionalParticles((l) => (this.isLinkHighlighted(l) ? PULSES_PER_FIBRE : 0))
      .linkDirectionalParticleSpeed(PULSE_SPEED)
      .linkDirectionalParticleWidth((l) => (this.isLinkHighlighted(l) ? PULSE_WIDTH * 1.8 : PULSE_WIDTH))
      .linkDirectionalParticleColor((l) => this.nodeColor(this.nodesById.get(idOf(l.source) ?? '')))
      .linkDirectionalParticleResolution(8)
      .onNodeHover((n) => this.hover(n))
      .onNodeClick((n) => this.click(n))
      // Lay out synchronously before the first frame, so the graph is
      // already spread out even if animation frames are throttled.
      .warmupTicks(WARMUP_TICKS)
      .cooldownTime(COOLDOWN_MS)
      .onEngineStop(() => {
        this.engineRunning = false;
        this.fit();
      });

    // Spread the layout so neurons (with their dendrites) do not crowd each
    // other: stronger repulsion for bigger neurons, and links long enough
    // to keep the two cell bodies apart.
    graph
      .d3Force('charge')
      ?.strength((n: Node) => -(CHARGE_BASE + CHARGE_PER_RADIUS * somaRadius(n)))
      .distanceMax(CHARGE_MAX_DISTANCE);
    graph
      .d3Force('link')
      ?.distance((l: Link) => LINK_BASE_DISTANCE + LINK_RADIUS_FACTOR * (somaRadius(this.nodeOf(l.source)) + somaRadius(this.nodeOf(l.target))));
    graph.d3Force('gravity', gravityForce());

    graph.lights(createLights());

    const renderer = graph.renderer();
    const pixelRatio = Math.min(window.devicePixelRatio, MAX_PIXEL_RATIO);
    renderer.setPixelRatio(pixelRatio);
    // No tone mapping: ACES darkened the background below #1e1e1e.

    const composer = graph.postProcessingComposer();
    composer.setPixelRatio(pixelRatio);
    this.bloomPass = new UnrealBloomPass(
      new Vector2(this.container.clientWidth, this.container.clientHeight),
      BLOOM.strength,
      BLOOM.radius,
      BLOOM.threshold,
    );
    this.bloomPass.enabled = this.look.bloom;
    composer.addPass(this.bloomPass);
    // Applies the sRGB conversion after the bloom pass.
    composer.addPass(new OutputPass());

    // With post-processing the clear colour is treated as linear and then
    // re-encoded to sRGB by OutputPass, which turns the dark background grey.
    // A scene background goes through colour management and stays exact.
    const scene = graph.scene();
    scene.background = new Color(this.look.background);
    scene.fog = new FogExp2(this.look.background, FOG_DEFAULT_DENSITY);

    // Any interaction wakes the render loop; it pauses again once idle.
    for (const event of ['pointerdown', 'pointermove', 'wheel', 'touchstart']) {
      this.container.addEventListener(event, () => this.wake(), { passive: true });
    }

    this.graph = graph;
  }

  /** Stops all rendering while the graph is not on screen. */
  hide(): void {
    window.clearTimeout(this.idleTimer);
    if (this.graph && !this.paused) {
      this.graph.pauseAnimation();
      this.paused = true;
    }
  }

  /** Resumes rendering and schedules a pause after IDLE_PAUSE_MS of quiet. */
  private wake(): void {
    if (!this.graph || !this.isVisible()) return;
    if (this.paused) {
      this.graph.resumeAnimation();
      this.paused = false;
    }
    window.clearTimeout(this.idleTimer);
    this.idleTimer = window.setTimeout(() => this.sleep(), IDLE_PAUSE_MS);
  }

  private sleep(): void {
    // Keep drawing while the layout moves or pulses run on a hovered neuron.
    if (this.engineRunning || this.hovered !== null) {
      this.wake();
      return;
    }
    this.hide();
  }

  /** Frames every node once the layout has settled after new data. */
  private fit(): void {
    if (!this.needsFit || !this.graph) return;
    this.needsFit = false;
    this.wake();
    this.fitFog();
    this.graph.zoomToFit(600, 40);
  }

  /** Scales the fog to the size of the laid-out graph. */
  private fitFog(): void {
    const fog = this.graph?.scene().fog;
    const box = this.graph?.getGraphBbox();
    if (!(fog instanceof FogExp2) || !box) return;
    const radius = Math.max(box.x[1] - box.x[0], box.y[1] - box.y[0], box.z[1] - box.z[0]) / 2;
    fog.density = radius > 0 ? FOG_SCALE / radius : FOG_DEFAULT_DENSITY;
  }

  private draw(): void {
    if (!this.graph || !this.pendingData) return;
    const data = this.pendingData;
    this.pendingData = null;

    this.assignColors(data.nodes);
    this.neighbors.clear();
    for (const link of data.links) {
      this.neighborsOf(link.source).add(link.target);
      this.neighborsOf(link.target).add(link.source);
    }
    for (const look of this.looks.values()) disposeLook(look);
    this.looks.clear();

    const nodes = data.nodes.map((n) => ({ ...n }));
    this.nodesById = new Map(nodes.map((n) => [n.id, n]));
    this.needsFit = true;
    this.engineRunning = true;
    this.wake();
    this.graph.graphData({
      nodes,
      links: data.links.map((l) => ({ source: l.source, target: l.target })),
    });
    // First framing on the warmed-up layout; fit() runs again on engine stop.
    window.setTimeout(() => {
      this.wake();
      this.fitFog();
      this.graph?.zoomToFit(400, 40);
    }, FIRST_FIT_DELAY_MS);
  }

  /** A neuron: glowing soma, organic dendrites and a dim title below. */
  private nodeObject(node: Node): Group {
    const radius = somaRadius(node);
    const shape = buildNeuron(hash(node.id), radius);
    const look: NeuronLook = {
      soma: new MeshStandardMaterial({ roughness: 0.55, metalness: 0 }),
      dendrites: new MeshStandardMaterial({ roughness: 0.75, metalness: 0 }),
      core: new SpriteMaterial({ map: coreTexture, blending: AdditiveBlending, depthWrite: false, transparent: true }),
      geometries: [shape.soma, shape.dendrites],
    };
    this.looks.set(node.id, look);
    this.paint(node, look);

    const core = new Sprite(look.core);
    core.scale.setScalar(radius * CORE_SCALE);

    const label = new SpriteText(node.title, LABEL_HEIGHT, this.look.label);
    label.fontSize = LABEL_FONT_PX;
    label.fontFace = 'Segoe UI, system-ui, sans-serif';
    label.fontWeight = '500';
    // Titles stay on top, readable and unaffected by fog.
    label.material.depthWrite = false;
    label.material.depthTest = false;
    label.material.fog = false;
    label.renderOrder = 10;
    const texture = label.material.map;
    if (texture && this.graph) texture.anisotropy = this.graph.renderer().capabilities.getMaxAnisotropy();
    label.position.y = -(radius + 3);

    const group = new Group();
    group.add(new Mesh(shape.dendrites, look.dendrites), new Mesh(shape.soma, look.soma), core, label);
    return group;
  }

  private paint(node: Node, look: NeuronLook): void {
    const dimmed = this.hovered !== null && node.id !== this.hovered && !this.neighbors.get(this.hovered)?.has(node.id);
    const base = new Color(dimmed ? this.look.dim : this.nodeColor(node));
    const firing = node.id === this.hovered;

    look.soma.color.copy(base);
    look.soma.emissive.copy(base);
    look.soma.emissiveIntensity = dimmed ? 0 : firing ? SOMA_GLOW * 2.2 : SOMA_GLOW;

    look.dendrites.color.copy(base).lerp(this.look.dendriteTint, DENDRITE_SHADE);
    look.dendrites.emissive.copy(base);
    look.dendrites.emissiveIntensity = dimmed ? 0 : firing ? 0.4 : 0.16;

    look.core.color.copy(base);
    look.core.opacity = dimmed ? 0 : firing ? CORE_OPACITY * 2.5 : CORE_OPACITY;
  }

  private nodeColor(node: Node | undefined): string {
    if (!node) return ROOT_COLOR;
    if (node.unresolved) return UNRESOLVED_COLOR;
    return this.colors.get(node.group) ?? ROOT_COLOR;
  }

  private fibreColor(link: Link): string {
    if (this.isLinkHighlighted(link)) return this.look.highlight;
    const source = this.nodesById.get(idOf(link.source) ?? '');
    return '#' + new Color(this.nodeColor(source)).lerp(this.look.dendriteTint, FIBRE_SHADE).getHexString();
  }

  private assignColors(nodes: GraphNodeData[]): void {
    this.colors = folderColors(nodes.filter((n) => !n.unresolved).map((n) => n.group));
  }

  private hover(node: Node | null): void {
    const id = node?.id ?? null;
    if (id === this.hovered) return;
    this.hovered = id;
    this.container.style.cursor = node ? 'pointer' : 'grab';
    for (const [nodeId, look] of this.looks) {
      const target = this.nodesById.get(nodeId);
      if (target) this.paint(target, look);
    }
    // Re-assigning the accessors makes the graph re-evaluate them.
    const graph = this.graph;
    graph
      ?.linkColor(graph.linkColor())
      .linkWidth(graph.linkWidth())
      .linkDirectionalParticleWidth(graph.linkDirectionalParticleWidth())
      .linkDirectionalParticles(graph.linkDirectionalParticles());
  }

  private click(node: Node): void {
    // Fly the camera towards the node, then open it.
    const { x = 0, y = 0, z = 0 } = node;
    const distance = 70;
    const ratio = 1 + distance / Math.max(Math.hypot(x, y, z), 1);
    this.wake();
    this.graph?.cameraPosition({ x: x * ratio, y: y * ratio, z: z * ratio }, { x, y, z }, 700);
    window.setTimeout(() => this.onOpen(node), 450);
  }

  private nodeOf(end: Link['source']): Node | undefined {
    return typeof end === 'object' ? end : this.nodesById.get(idOf(end) ?? '');
  }

  private isLinkHighlighted(link: Link): boolean {
    if (!this.hovered) return false;
    return idOf(link.source) === this.hovered || idOf(link.target) === this.hovered;
  }

  private neighborsOf(id: string): Set<string> {
    let set = this.neighbors.get(id);
    if (!set) {
      set = new Set();
      this.neighbors.set(id, set);
    }
    return set;
  }

  private resize(): void {
    if (!this.graph || !this.isVisible()) return;
    this.graph.width(this.container.clientWidth).height(this.container.clientHeight);
    this.wake();
  }

  private isVisible(): boolean {
    return this.container.offsetParent !== null;
  }
}

/** Soma radius: grows with the square root of the link count, capped. */
function somaRadius(node: Node | undefined): number {
  const degree = node?.degree ?? 0;
  return Math.min(SOMA_BASE_RADIUS + Math.sqrt(degree) * SOMA_DEGREE_RADIUS, SOMA_MAX_RADIUS);
}

/** Pulls every node gently towards the origin; orphans harder. */
function gravityForce() {
  let nodes: Node[] = [];
  const force = (alpha: number) => {
    for (const n of nodes) {
      const k = (n.degree === 0 ? ORPHAN_GRAVITY : GRAVITY) * alpha;
      n.vx = (n.vx ?? 0) - (n.x ?? 0) * k;
      n.vy = (n.vy ?? 0) - (n.y ?? 0) * k;
      n.vz = (n.vz ?? 0) - (n.z ?? 0) * k;
    }
  };
  force.initialize = (all: Node[]) => {
    nodes = all;
  };
  return force;
}

/** Dim fill plus a warm key light, so dendrites read as lit tissue. */
function createLights() {
  const key = new DirectionalLight(0xffe7d6, 1.6);
  key.position.set(1, 1.2, 1);
  const fill = new DirectionalLight(0x9fb4ff, 0.45);
  fill.position.set(-1, -0.4, -0.8);
  return [new AmbientLight(0xffffff, 0.5), key, fill];
}

/** A white radial gradient, tinted per neuron through the sprite colour. */
function createGlowTexture(): CanvasTexture {
  const size = 128;
  const canvas = document.createElement('canvas');
  canvas.width = canvas.height = size;
  const ctx = canvas.getContext('2d');
  if (ctx) {
    const gradient = ctx.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
    gradient.addColorStop(0, 'rgba(255,255,255,1)');
    gradient.addColorStop(0.25, 'rgba(255,255,255,0.5)');
    gradient.addColorStop(0.6, 'rgba(255,255,255,0.08)');
    gradient.addColorStop(1, 'rgba(255,255,255,0)');
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, size, size);
  }
  return new CanvasTexture(canvas);
}

function disposeLook(look: NeuronLook): void {
  look.soma.dispose();
  look.dendrites.dispose();
  look.core.dispose();
  look.geometries.forEach((g) => g.dispose());
}

function idOf(end: Link['source']): string | undefined {
  return typeof end === 'object' ? end.id : end?.toString();
}
