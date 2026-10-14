// 3D graph view. Drag rotates the sphere of notes (orbit controls), the
// wheel zooms, right-drag pans, clicking a node opens it.
//
// Rendering quality: the canvas is rendered above the screen's pixel ratio
// (supersampling) with antialiasing, nodes are lit physically based spheres,
// links are tubes instead of 1px lines and a soft bloom pass adds glow.

import ForceGraph3D, { type ConfigOptions, type ForceGraph3DInstance, type LinkObject, type NodeObject } from '3d-force-graph';
import SpriteText from 'three-spritetext';
import {
  ACESFilmicToneMapping,
  AmbientLight,
  Color,
  DirectionalLight,
  Group,
  HemisphereLight,
  Mesh,
  MeshStandardMaterial,
  type Object3D,
  SphereGeometry,
  Vector2,
  Vector3,
} from 'three';
import { helixGeometry, helixLength, turnsFor } from './helix';

type Coords = { x: number; y: number; z: number };
import { UnrealBloomPass } from 'three/examples/jsm/postprocessing/UnrealBloomPass.js';
import { OutputPass } from 'three/examples/jsm/postprocessing/OutputPass.js';
import type { GraphData, GraphNodeData } from '../api/api';

type Node = NodeObject & GraphNodeData;
type Link = LinkObject<Node>;

// The default export is not generic over node/link types; narrow it once here.
const Graph3D = ForceGraph3D as unknown as new (element: HTMLElement, config?: ConfigOptions) => ForceGraph3DInstance<Node, Link>;

const palette = ['#a855f7', '#ef4444', '#f97316', '#eab308', '#10b981', '#3b82f6', '#ec4899', '#14b8a6', '#84cc16', '#f43f5e'];
const rootColor = '#a1a1aa';
const unresolvedColor = '#52525b';
const dimColor = '#3a3a40';
const background = '#1e1e1e';

// Links are drawn as DNA-like double helices: light grey strands that read
// clearly on the dark background, with dimmer rungs between them.
const LINK_COLOR = '#a8a8b0';
const strandMaterial = linkMaterial(LINK_COLOR, 0.15);
const rungMaterial = linkMaterial('#6e6e7a', 0.1);
const strandHighlight = linkMaterial('#f4f4f5', 0.45);
const rungHighlight = linkMaterial('#b8b0ff', 0.4);

const NODE_REL_SIZE = 4;
const WARMUP_TICKS = 120;
const COOLDOWN_MS = 3000;
// Delay before the first framing: enough for the graph to apply the data
// and run the warm-up ticks.
const FIRST_FIT_DELAY_MS = 300;

// Render at twice the screen density (capped) so edges and text stay crisp.
const SUPERSAMPLE = 2;
const MAX_PIXEL_RATIO = 3;

// Label texture resolution in px; higher means sharper text when zoomed in.
const LABEL_FONT_PX = 160;

const BLOOM = { strength: 0.45, radius: 0.4, threshold: 0.72 };

// One unit sphere shared by every node; each mesh is scaled to its radius.
const sphere = new SphereGeometry(1, 48, 32);

export class GraphView {
  private graph: ForceGraph3DInstance<Node, Link> | null = null;
  private colors = new Map<string, string>();
  private neighbors = new Map<string, Set<string>>();
  private materials = new Map<string, MeshStandardMaterial>();
  private linkMeshes = new Map<Link, Mesh>();
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
      .nodeRelSize(NODE_REL_SIZE)
      .nodeVal((n) => 1 + n.degree)
      .nodeLabel((n) => n.id)
      .nodeThreeObject((n) => this.nodeObject(n))
      .linkThreeObject((l) => this.linkObject(l))
      .linkPositionUpdate((obj, { start, end }) => placeHelix(obj, start, end))
      .linkDirectionalArrowColor(() => LINK_COLOR)
      .linkDirectionalArrowLength(1.8)
      .linkDirectionalArrowRelPos(1)
      .linkDirectionalArrowResolution(16)
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
    renderer.toneMappingExposure = 1.05;

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
    graph.scene().background = new Color(background);

    this.graph = graph;
  }

  /** Frames every node once the layout has settled after new data. */
  private fit(): void {
    if (!this.needsFit || !this.graph) return;
    this.needsFit = false;
    this.graph.zoomToFit(600, 60);
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
    for (const material of this.materials.values()) material.dispose();
    this.materials.clear();
    this.linkMeshes.clear();

    const nodes = data.nodes.map((n) => ({ ...n }));
    this.nodesById = new Map(nodes.map((n) => [n.id, n]));
    this.needsFit = true;
    this.graph.graphData({
      nodes,
      links: data.links.map((l) => ({ source: l.source, target: l.target })),
    });
    // First framing on the warmed-up layout; fit() runs again on engine stop.
    window.setTimeout(() => this.graph?.zoomToFit(400, 60), FIRST_FIT_DELAY_MS);
  }

  /** A lit sphere sized by degree, with its title floating below it. */
  private nodeObject(node: Node): Group {
    const radius = Math.cbrt(1 + node.degree) * NODE_REL_SIZE;
    const material = new MeshStandardMaterial({ roughness: 0.35, metalness: 0.1 });
    this.materials.set(node.id, material);
    this.paint(node, material);

    const ball = new Mesh(sphere, material);
    ball.scale.setScalar(radius);

    const label = new SpriteText(node.title, 2.6, '#c8c8ce');
    label.fontSize = LABEL_FONT_PX;
    label.fontFace = 'Segoe UI, system-ui, sans-serif';
    label.fontWeight = '500';
    // Draw titles on top of everything so helices never cut through them.
    label.material.depthWrite = false;
    label.material.depthTest = false;
    label.renderOrder = 10;
    const texture = label.material.map;
    if (texture && this.graph) texture.anisotropy = this.graph.renderer().capabilities.getMaxAnisotropy();
    label.position.y = -(radius + 3);

    const group = new Group();
    group.add(ball, label);
    return group;
  }

  /** A double helix; placeHelix positions and sizes it every tick. */
  private linkObject(link: Link): Mesh {
    const mesh = new Mesh(helixGeometry(1), [strandMaterial, rungMaterial]);
    this.linkMeshes.set(link, mesh);
    return mesh;
  }

  private paint(node: Node, material: MeshStandardMaterial): void {
    const dimmed = this.hovered !== null && node.id !== this.hovered && !this.neighbors.get(this.hovered)?.has(node.id);
    const base = dimmed ? dimColor : node.unresolved ? unresolvedColor : this.colors.get(node.group) ?? rootColor;
    material.color.set(base);
    // A little self-illumination keeps colours vivid on the unlit side and
    // feeds the bloom pass; the hovered node glows more.
    material.emissive.copy(new Color(base));
    material.emissiveIntensity = dimmed ? 0 : node.id === this.hovered ? 0.55 : 0.18;
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
    for (const [nodeId, material] of this.materials) {
      const target = this.nodesById.get(nodeId);
      if (target) this.paint(target, material);
    }
    for (const [link, mesh] of this.linkMeshes) {
      const lit = this.isLinkHighlighted(link);
      mesh.material = lit ? [strandHighlight, rungHighlight] : [strandMaterial, rungMaterial];
    }
  }

  private click(node: Node): void {
    // Fly the camera towards the node, then open it.
    const { x = 0, y = 0, z = 0 } = node;
    const distance = 80;
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

/** Soft ambient fill plus a key and a tinted rim light for depth. */
function createLights() {
  const key = new DirectionalLight(0xffffff, 1.8);
  key.position.set(1, 1.2, 1);
  const rim = new DirectionalLight(0x8b7cf6, 0.6);
  rim.position.set(-1, -0.6, -1);
  return [new AmbientLight(0xffffff, 0.35), new HemisphereLight(0xdfe3ff, 0x202024, 0.6), key, rim];
}

function linkMaterial(color: string, glow: number): MeshStandardMaterial {
  return new MeshStandardMaterial({ color, emissive: color, emissiveIntensity: glow, roughness: 0.45, metalness: 0.1 });
}

const forward = new Vector3();

/**
 * Stretches a helix mesh between two nodes: centred at the midpoint, its +Z
 * axis towards the target, with as many turns as the length calls for.
 * Returns true so the graph skips its default line placement.
 */
function placeHelix(obj: Object3D, start: Coords, end: Coords): boolean {
  const mesh = obj as Mesh;
  forward.set(end.x - start.x, end.y - start.y, end.z - start.z);
  const length = forward.length();
  if (length === 0) return true;

  const turns = turnsFor(length);
  const geometry = helixGeometry(turns);
  if (mesh.geometry !== geometry) mesh.geometry = geometry;

  mesh.position.set((start.x + end.x) / 2, (start.y + end.y) / 2, (start.z + end.z) / 2);
  mesh.lookAt(end.x, end.y, end.z);
  mesh.scale.set(1, 1, length / helixLength(turns));
  return true;
}

function idOf(end: Link['source']): string | undefined {
  return typeof end === 'object' ? end.id : end?.toString();
}
