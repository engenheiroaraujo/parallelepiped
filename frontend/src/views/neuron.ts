// Procedural neuron geometry for the graph: an irregular cell body (soma)
// and organic dendrites that wave, taper and branch.
//
// Shapes are derived from a seed, so a note keeps the same neuron across
// redraws. Each neuron is two meshes: the soma and all of its dendrites
// merged into one geometry (one draw call per neuron).

import { BufferAttribute, BufferGeometry, CatmullRomCurve3, IcosahedronGeometry, TubeGeometry, Vector3 } from 'three';
import { mergeGeometries, mergeVertices } from 'three/examples/jsm/utils/BufferGeometryUtils.js';

const MIN_DENDRITES = 5;
const MAX_DENDRITES = 8;
const DENDRITE_LENGTH = [3.5, 7] as const; // in soma radii
const DENDRITE_POINTS = 7;
const DENDRITE_WAVINESS = 0.45;
const DENDRITE_BASE = 0.3; // base radius, in soma radii
const DENDRITE_TIP = 0.025; // tip radius, in soma radii
const BRANCH_CHANCE = 0.75;
// Geometry detail, kept modest: hundreds of neurons share the GPU.
const TUBE_SEGMENTS = 16;
const TUBE_SIDES = 4;
const SOMA_DETAIL = 3;
const SOMA_ROUGHNESS = 0.14;

export interface NeuronGeometry {
  soma: BufferGeometry;
  dendrites: BufferGeometry;
}

/** Builds a neuron of the given soma radius from a [0, 1) seed. */
export function buildNeuron(seed: number, radius: number): NeuronGeometry {
  const random = seeded(seed);
  const soma = buildSoma(random, radius);

  const tubes: BufferGeometry[] = [];
  const count = MIN_DENDRITES + Math.floor(random() * (MAX_DENDRITES - MIN_DENDRITES + 1));
  for (let i = 0; i < count; i++) {
    const direction = randomDirection(random);
    const length = radius * lerp(DENDRITE_LENGTH[0], DENDRITE_LENGTH[1], random());
    const start = direction.clone().multiplyScalar(radius * 0.8);
    const points = wavyPath(random, start, direction, length);
    tubes.push(taperedTube(points, radius * DENDRITE_BASE, radius * DENDRITE_TIP));

    // A thinner side branch leaving from the middle of the dendrite.
    if (random() < BRANCH_CHANCE) {
      const from = 2 + Math.floor(random() * 2);
      const along = points[from + 1].clone().sub(points[from]).normalize();
      const branchDirection = along.add(randomDirection(random).multiplyScalar(0.9)).normalize();
      const branch = wavyPath(random, points[from], branchDirection, length * lerp(0.35, 0.6, random()));
      const thickness = radius * lerp(DENDRITE_BASE, DENDRITE_TIP, from / (DENDRITE_POINTS - 1)) * 0.7;
      tubes.push(taperedTube(branch, thickness, radius * DENDRITE_TIP));
    }
  }

  const dendrites = mergeGeometries(tubes);
  tubes.forEach((tube) => tube.dispose());
  return { soma, dendrites };
}

/** A sphere with a lumpy surface, so cell bodies do not look like balls. */
function buildSoma(random: () => number, radius: number): BufferGeometry {
  // Icosahedron faces do not share vertices; merging them lets the normals
  // be averaged, so the surface shades smoothly instead of faceted.
  const raw = new IcosahedronGeometry(radius, SOMA_DETAIL);
  raw.deleteAttribute('normal');
  raw.deleteAttribute('uv');
  const geometry = mergeVertices(raw);
  raw.dispose();
  const bumps = Array.from({ length: 4 }, () => ({ axis: randomDirection(random), phase: random() * Math.PI * 2, freq: 2 + random() * 3 }));
  const position = geometry.attributes.position as BufferAttribute;
  const v = new Vector3();
  for (let i = 0; i < position.count; i++) {
    v.fromBufferAttribute(position, i);
    const unit = v.clone().normalize();
    let bump = 0;
    for (const b of bumps) bump += Math.sin(unit.dot(b.axis) * b.freq + b.phase);
    v.multiplyScalar(1 + (bump / bumps.length) * SOMA_ROUGHNESS);
    position.setXYZ(i, v.x, v.y, v.z);
  }
  position.needsUpdate = true;
  geometry.computeVertexNormals();
  return geometry;
}

/** Points that drift away from start, bending a little at each step. */
function wavyPath(random: () => number, start: Vector3, direction: Vector3, length: number): Vector3[] {
  const step = length / (DENDRITE_POINTS - 1);
  const heading = direction.clone();
  const points = [start.clone()];
  for (let i = 1; i < DENDRITE_POINTS; i++) {
    heading.add(randomDirection(random).multiplyScalar(DENDRITE_WAVINESS)).normalize();
    points.push(points[i - 1].clone().addScaledVector(heading, step));
  }
  return points;
}

/** A tube along a smooth curve through points, narrowing from base to tip. */
function taperedTube(points: Vector3[], base: number, tip: number): BufferGeometry {
  const curve = new CatmullRomCurve3(points);
  const tube = new TubeGeometry(curve, TUBE_SEGMENTS, 1, TUBE_SIDES, false);
  const position = tube.attributes.position as BufferAttribute;
  const center = new Vector3();
  const v = new Vector3();
  // TubeGeometry lays out (TUBE_SIDES + 1) vertices per ring along the curve.
  for (let ring = 0; ring <= TUBE_SEGMENTS; ring++) {
    const t = ring / TUBE_SEGMENTS;
    curve.getPointAt(t, center);
    // Thin out faster near the tip, like real dendrites.
    const width = lerp(base, tip, Math.pow(t, 0.7));
    for (let side = 0; side <= TUBE_SIDES; side++) {
      const index = ring * (TUBE_SIDES + 1) + side;
      v.fromBufferAttribute(position, index).sub(center).multiplyScalar(width).add(center);
      position.setXYZ(index, v.x, v.y, v.z);
    }
  }
  position.needsUpdate = true;
  tube.computeVertexNormals();
  return tube;
}

function randomDirection(random: () => number): Vector3 {
  // Uniform on the sphere.
  const z = random() * 2 - 1;
  const angle = random() * Math.PI * 2;
  const r = Math.sqrt(1 - z * z);
  return new Vector3(r * Math.cos(angle), r * Math.sin(angle), z);
}

function lerp(a: number, b: number, t: number): number {
  return a + (b - a) * t;
}

/** Small deterministic PRNG (mulberry32) seeded from a [0, 1) value. */
export function seeded(seed: number): () => number {
  let state = Math.floor(seed * 4294967296);
  return () => {
    state = (state + 0x6d2b79f5) | 0;
    let t = Math.imul(state ^ (state >>> 15), 1 | state);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Stable hash of a string to [0, 1). */
export function hash(text: string): number {
  let h = 2166136261;
  for (let i = 0; i < text.length; i++) {
    h ^= text.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return (h >>> 0) / 4294967296;
}
