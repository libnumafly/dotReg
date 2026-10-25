// Share the JavaScript engine so grammar and performance fixes cannot drift.
export { DotRegError, parse, parseEach, decodeString } from '../javascript/dotreg.mjs';
export type { Entry, Text, Source, ValueForm } from '../javascript/dotreg.mjs';
