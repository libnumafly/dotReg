export type ValueForm = '' | 'triple_quoted' | 'pipe';
export interface Entry {
  line: number;
  kind: 'heading' | 'value';
  path: string[];
  raw: string;
  unset: boolean;
  has_type: boolean;
  type?: string;
  form?: ValueForm;
}
export interface Text { value: string; unset: boolean }
export type Source = string | Uint8Array | Iterable<Uint8Array> | AsyncIterable<Uint8Array>;
export class DotRegError extends Error {
  readonly line: number;
  readonly code: string;
  constructor(line: number, code?: string);
}
export function parseEach(source: Source, visit: (entry: Entry) => void | boolean | Promise<void | boolean>): Promise<void>;
export function parse(source: Source): Promise<Entry[]>;
export function decodeString(entry: Entry): Text;
