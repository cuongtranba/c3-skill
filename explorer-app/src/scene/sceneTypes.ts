import * as THREE from "three";
import type { C3Edge, C3Node } from "../data";

export type NodeMesh = THREE.Mesh<THREE.BufferGeometry, THREE.MeshStandardMaterial>;
export type BasicMesh = THREE.Mesh<THREE.BufferGeometry, THREE.MeshBasicMaterial>;

export type LNode = C3Node & {
  _x?: number;
  _y?: number;
  _z?: number;
  _hgt?: number;
  _want?: number;
  _grp?: THREE.Group;
  _mesh?: NodeMesh;
  _mat?: THREE.MeshStandardMaterial;
  _searchHit?: boolean;
  _bornAt?: number;
  _flashUntil?: number;
  _pulseUntil?: number;
  _pulseColor?: string;
};

export interface EdgeRec {
  e: C3Edge;
  curve: THREE.Curve<THREE.Vector3>;
  tube: BasicMesh;
  arrowMeshes: BasicMesh[];
  particles: BasicMesh[];
  gates: BasicMesh[];
  kind: string;
  from: string;
  to: string;
  baseOpacity: number;
  crossedLabels: string[];
}
