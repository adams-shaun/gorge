import type { Decision, PaymentPlan, View } from '../../protocol';

/** The SpellBench v2 HTTP seat's documents, as far as this client reads them (docs/site/api/spellbench-v2.md). */

/** NativeOp is what a candidate means natively: x_gorge_view_v1's ops[i] belongs to candidate i. */
export interface NativeOp {
  op: string;
  option: number;
  payment?: { action_id: string; plan: PaymentPlan };
  inside?: boolean;
  list?: string;
  position?: number;
  unit?: number;
  covers?: number[];
}

export interface ViewPayload {
  version: number;
  native_index: number;
  view: View;
  decision: Decision;
  /** The later native decisions a candidate of this pose also answers, by the key its ops name. */
  followups?: Record<string, Decision>;
  ops: NativeOp[] | null;
}

export interface HumanExtension {
  log: string[];
  labels: Record<string, string>;
  can_undo: boolean;
  native_only: boolean;
}

export interface Candidate {
  candidate_id: number;
  semantic: { kind: string } & Record<string, unknown>;
}

export interface Sbv2Request {
  request_id: string;
  decision: {
    group?: { substep_index: number };
    candidates?: Candidate[] | null;
    extensions: { x_gorge_view_v1: ViewPayload; x_gorge_human_v1: HumanExtension };
  };
}

/** The body of POST .../send. */
export type Sbv2Send =
  | { request_id: string; candidate_id: number }
  | { request_id: string; x_gorge_human_v1: { concede: true } | { undo: true } | { choose_name: string } };
