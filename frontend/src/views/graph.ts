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
  ACESFilmicToneMapping,
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
import { buildNeuron, hash } from './neuron';

type Node = NodeObject & GraphNodeData;
type Link = LinkObject<Node>;

// The default export is not generic over node/link types; narrow it once here.
const Graph3D = ForceGraph3D as unknown as new (element: HTMLElement, config?: ConfigOptions) => ForceGraph3DInstance<Node, Link>;

// Folder colours (the same scheme as before), one per top-level folder.
const palette = ['#a855f7', '#ef4444', '#f97316', '#eab308', '#10b981', '#3b82f6', '#ec4899', '#14b8a6', '#84cc16', '#f43f5e'];
const rootColor = '#a1a1aa';
const unresolvedColor = '#52525b';
const dimColor = '#2c2c32';
const background = '#1e1e1e';

// Neurons.
// Like Obsidian, a note with more links is bigger: the radius grows with the
// square root of its link count, capped so hubs do not swallow the view.
const SOMA_BASE_RADIUS = 1.2;
const SOMA_DEGREE_RADIUS = 0.9;
const SOMA_MAX_RADIUS = 12;
const SOMA_GLOW = 0.35; // emissive intensity of the cell body
const DENDRITE_SHADE = 0.55; // how far dendrites darken towards DENDRITE_TINT
const DENDRITE_TINT = new Color('#2a201c');
const CORE_SCALE = 2.4; // inner glow diameter, in soma radii
const CORE_OPACITY = 0.3;

// Fibres between neurons.
const FIBRE_SHADE = 0.45;
const FIBRE_OPACITY = 0.6;
const FIBRE_WIDTH = 0.22;
const FIBRE_CURVATURE = 0.2;
const PULSES_PER_FIBRE = 1;
const PULSE_SPEED = 0.004;
const PULSE_WIDTH = 0.55;

// Exponential fog density; distant neurons fade into the background.
const FOG_DENSITY = 0.0028;

// Labels: dim enough to stay under the bloom threshold.
const LABEL_COLOR = '#9a9aa4';
const LABEL_HEIGHT = 2.4;
// Label texture resolution in px; higher means sharper text when zoomed in.
const LABEL_FONT_PX = 160;

const WARMUP_TICKS = 120;
const COOLDOWN_MS = 3000;
// Delay before the first framing: enough for the graph to apply the data
// and run the warm-up ticks.
const FIRST_FIT_DELAY_MS = 300;

// Render at twice the screen density (capped) so fibres and text stay crisp.
const SUPERSAMPLE = 2;
const MAX_PIXEL_RATIO = 3;

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

  constructor(
    private readonly container: HTMLElement,
    private readonly onOpen: (node: GraphNodeData) => void,
  ) {
    new ResizeObserver(() => this.resize()).observe(container);
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
      .backgroundColor(background)
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
      .linkDirectionalParticles(PULSES_PER_FIBRE)
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
      .onEngineStop(() => this.fit());

    graph.lights(createLights());

    const renderer = graph.renderer();
    const pixelRatio = Math.min(window.devicePixelRatio * SUPERSAMPLE, MAX_PIXEL_RATIO);
    renderer.setPixelRatio(pixelRatio);
    renderer.toneMapping = ACESFilmicToneMapping;

    const composer = graph.postProcessingComposer();
    composer.setPixelRatio(pixelRatio);
    composer.addPass(
      new UnrealBloomPass(
        new Vector2(this.container.clientWidth, this.container.clientHeight),
        BLOOM.strength,
        BLOOM.radius,
        BLOOM.threshold,
      ),
    );
    // Applies tone mapping and sRGB conversion after the bloom pass.
    composer.addPass(new OutputPass());

    // With post-processing the clear colour is treated as linear and then
    // re-encoded to sRGB by OutputPass, which turns the dark background grey.
    // A scene background goes through colour management and stays exact.
    const scene = graph.scene();
    scene.background = new Color(background);
    scene.fog = new FogExp2(background, FOG_DENSITY);

    this.graph = graph;
  }

  /** Frames every node once the layout has settled after new data. */
  private fit(): void {
    if (!this.needsFit || !this.graph) return;
    this.needsFit = false;
    this.graph.zoomToFit(600, 40);
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
    this.graph.graphData({
      nodes,
      links: data.links.map((l) => ({ source: l.source, target: l.target })),
    });
    // First framing on the warmed-up layout; fit() runs again on engine stop.
    window.setTimeout(() => this.graph?.zoomToFit(400, 40), FIRST_FIT_DELAY_MS);
  }

  /** A neuron: glowing soma, organic dendrites and a dim title below. */
  private nodeObject(node: Node): Group {
    const radius = Math.min(SOMA_BASE_RADIUS + Math.sqrt(node.degree) * SOMA_DEGREE_RADIUS, SOMA_MAX_RADIUS);
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

    const label = new SpriteText(node.title, LABEL_HEIGHT, LABEL_COLOR);
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
    const base = new Color(dimmed ? dimColor : this.nodeColor(node));
    const firing = node.id === this.hovered;

    look.soma.color.copy(base);
    look.soma.emissive.copy(base);
    look.soma.emissiveIntensity = dimmed ? 0 : firing ? SOMA_GLOW * 2.2 : SOMA_GLOW;

    look.dendrites.color.copy(base).lerp(DENDRITE_TINT, DENDRITE_SHADE);
    look.dendrites.emissive.copy(base);
    look.dendrites.emissiveIntensity = dimmed ? 0 : firing ? 0.35 : 0.08;

    look.core.color.copy(base);
    look.core.opacity = dimmed ? 0 : firing ? CORE_OPACITY * 2.5 : CORE_OPACITY;
  }

  private nodeColor(node: Node | undefined): string {
    if (!node) return rootColor;
    if (node.unresolved) return unresolvedColor;
    return this.colors.get(node.group) ?? rootColor;
  }

  private fibreColor(link: Link): string {
    if (this.isLinkHighlighted(link)) return '#f4f4f5';
    const source = this.nodesById.get(idOf(link.source) ?? '');
    return '#' + new Color(this.nodeColor(source)).lerp(DENDRITE_TINT, FIBRE_SHADE).getHexString();
  }

  private assignColors(nodes: GraphNodeData[]): void {
    const groups = [...new Set(nodes.filter((n) => !n.unresolved && n.group !== '').map((n) => n.group))].sort();
    this.colors = new Map(groups.map((group, i) => [group, palette[i % palette.length]]));
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
      .linkDirectionalParticleWidth(graph.linkDirectionalParticleWidth());
  }

  private click(node: Node): void {
    // Fly the camera towards the node, then open it.
    const { x = 0, y = 0, z = 0 } = node;
    const distance = 70;
    const ratio = 1 + distance / Math.max(Math.hypot(x, y, z), 1);
    this.graph?.cameraPosition({ x: x * ratio, y: y * ratio, z: z * ratio }, { x, y, z }, 700);
    window.setTimeout(() => this.onOpen(node), 450);
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
  }

  private isVisible(): boolean {
    return this.container.offsetParent !== null;
  }
}

/** Dim fill plus a warm key light, so dendrites read as lit tissue. */
function createLights() {
  const key = new DirectionalLight(0xffe7d6, 1.6);
  key.position.set(1, 1.2, 1);
  const fill = new DirectionalLight(0x9fb4ff, 0.45);
  fill.position.set(-1, -0.4, -0.8);
  return [new AmbientLight(0xffffff, 0.3), key, fill];
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
