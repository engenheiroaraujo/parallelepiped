// 3D graph view. Drag rotates the sphere of notes (orbit controls), the
// wheel zooms, right-drag pans, clicking a node opens it.

import ForceGraph3D, { type ConfigOptions, type ForceGraph3DInstance, type LinkObject, type NodeObject } from '3d-force-graph';
import SpriteText from 'three-spritetext';
import type { GraphData, GraphNodeData } from '../api/api';

type Node = NodeObject & GraphNodeData;
type Link = LinkObject<Node>;

// The default export is not generic over node/link types; narrow it once here.
const Graph3D = ForceGraph3D as unknown as new (element: HTMLElement, config?: ConfigOptions) => ForceGraph3DInstance<Node, Link>;

const palette = ['#a855f7', '#ef4444', '#f97316', '#eab308', '#10b981', '#3b82f6', '#ec4899', '#14b8a6', '#84cc16', '#f43f5e'];
const rootColor = '#a1a1aa';
const unresolvedColor = '#52525b';
const dimColor = '#3f3f46';

const NODE_REL_SIZE = 4;
const WARMUP_TICKS = 120;
const COOLDOWN_MS = 3000;
// Delay before the first framing: enough for the graph to apply the data
// and run the warm-up ticks.
const FIRST_FIT_DELAY_MS = 300;

export class GraphView {
  private graph: ForceGraph3DInstance<Node, Link> | null = null;
  private colors = new Map<string, string>();
  private neighbors = new Map<string, Set<string>>();
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
    this.graph = new Graph3D(this.container, { controlType: 'orbit' })
      .backgroundColor('#1e1e1e')
      .showNavInfo(false)
      .nodeRelSize(NODE_REL_SIZE)
      .nodeVal((n) => 1 + n.degree)
      .nodeLabel((n) => n.id)
      .nodeColor((n) => this.nodeColor(n))
      .nodeOpacity(0.95)
      .nodeResolution(16)
      .nodeThreeObjectExtend(true)
      .nodeThreeObject((n) => this.label(n))
      .linkColor((l) => (this.isLinkHighlighted(l) ? '#d4d4d8' : '#5b5b63'))
      .linkOpacity(0.35)
      .linkWidth((l) => (this.isLinkHighlighted(l) ? 0.8 : 0))
      .linkDirectionalArrowLength(2.5)
      .linkDirectionalArrowRelPos(1)
      .onNodeHover((n) => this.hover(n))
      .onNodeClick((n) => this.click(n))
      // Lay out synchronously before the first frame, so the graph is
      // already spread out even if animation frames are throttled.
      .warmupTicks(WARMUP_TICKS)
      .cooldownTime(COOLDOWN_MS)
      .onEngineStop(() => this.fit());  }

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
    this.needsFit = true;
    this.graph.graphData({
      nodes: data.nodes.map((n) => ({ ...n })),
      links: data.links.map((l) => ({ source: l.source, target: l.target })),
    });
    // First framing on the warmed-up layout; fit() runs again on engine stop.
    window.setTimeout(() => this.graph?.zoomToFit(400, 60), FIRST_FIT_DELAY_MS);
  }

  private label(node: Node): SpriteText {
    const sprite = new SpriteText(node.title, 2.6, '#d4d4d8');
    sprite.material.depthWrite = false;
    const radius = Math.cbrt(1 + node.degree) * NODE_REL_SIZE;
    sprite.position.y = -(radius + 3);
    return sprite;
  }

  private nodeColor(node: Node): string {
    if (this.hovered && node.id !== this.hovered && !this.neighbors.get(this.hovered)?.has(node.id)) {
      return dimColor;
    }
    if (node.unresolved) return unresolvedColor;
    return this.colors.get(node.group) ?? rootColor;
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
    // Re-assigning the accessors makes the graph re-evaluate them.
    this.graph?.nodeColor(this.graph.nodeColor()).linkColor(this.graph.linkColor()).linkWidth(this.graph.linkWidth());
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

function idOf(end: Link['source']): string | undefined {
  return typeof end === 'object' ? end.id : end?.toString();
}
