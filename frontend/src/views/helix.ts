// Double-helix (DNA-like) link geometry for the 3D graph.
//
// Each geometry spans `turns * HELIX_PITCH` along +Z, centred on the origin,
// with two strands wound around the axis and "base pair" rungs between them.
// A link mesh is placed at the midpoint, turned towards the target and scaled
// on Z by length / helixLength(turns). turnsFor keeps that factor close to 1,
// so strands are barely stretched; rungs lie in the XY plane and never are.
// Geometries are cached by turn count and shared by all links.

import { BufferGeometry, CatmullRomCurve3, CylinderGeometry, TubeGeometry, Vector3 } from 'three';
import { mergeGeometries } from 'three/examples/jsm/utils/BufferGeometryUtils.js';

/** Distance between the axis and each strand. */
export const HELIX_RADIUS = 0.55;
/** Link length covered by one full turn. */
export const HELIX_PITCH = 8;

const STRAND_THICKNESS = 0.09;
const RUNG_THICKNESS = 0.04;
const RUNGS_PER_TURN = 5;
const SAMPLES_PER_TURN = 24;
const MAX_TURNS = 24;

/** Material slots of the merged geometry. */
export const STRAND_SLOT = 0;
export const RUNG_SLOT = 1;

const cache = new Map<number, BufferGeometry>();

/** Number of turns a link of the given length should show. */
export function turnsFor(length: number): number {
  return Math.min(MAX_TURNS, Math.max(1, Math.round(length / HELIX_PITCH)));
}

/** Length along Z of the geometry with the given number of turns. */
export function helixLength(turns: number): number {
  return turns * HELIX_PITCH;
}

/** The shared helix geometry with the given number of turns. */
export function helixGeometry(turns: number): BufferGeometry {
  let geometry = cache.get(turns);
  if (!geometry) {
    geometry = buildHelix(turns);
    cache.set(turns, geometry);
  }
  return geometry;
}

function buildHelix(turns: number): BufferGeometry {
  const strands = [0, Math.PI].map((phase) => strand(turns, phase));
  const rungs: BufferGeometry[] = [];
  const count = turns * RUNGS_PER_TURN;
  for (let i = 1; i < count; i++) {
    const t = i / count;
    const angle = 2 * Math.PI * turns * t;
    const rung = new CylinderGeometry(RUNG_THICKNESS, RUNG_THICKNESS, HELIX_RADIUS * 2, 6, 1);
    // Cylinders run along Y; turn them to point at the strands' angle.
    rung.rotateZ(angle - Math.PI / 2);
    rung.translate(0, 0, (t - 0.5) * helixLength(turns));
    rungs.push(rung);
  }

  // Two groups: strands use material slot 0, rungs slot 1.
  const strandGeometry = mergeGeometries(strands);
  const rungGeometry = rungs.length > 0 ? mergeGeometries(rungs) : null;
  const parts = rungGeometry ? [strandGeometry, rungGeometry] : [strandGeometry];
  const merged = mergeGeometries(parts, true);

  [...strands, ...rungs, strandGeometry, ...(rungGeometry ? [rungGeometry] : [])].forEach((g) => g.dispose());
  return merged;
}

function strand(turns: number, phase: number): BufferGeometry {
  const samples = turns * SAMPLES_PER_TURN;
  const points: Vector3[] = [];
  for (let i = 0; i <= samples; i++) {
    const t = i / samples;
    const angle = 2 * Math.PI * turns * t + phase;
    points.push(new Vector3(HELIX_RADIUS * Math.cos(angle), HELIX_RADIUS * Math.sin(angle), (t - 0.5) * helixLength(turns)));
  }
  return new TubeGeometry(new CatmullRomCurve3(points), samples, STRAND_THICKNESS, 6, false);
}
