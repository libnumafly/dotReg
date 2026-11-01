// Share the JavaScript engine so grammar and performance fixes cannot drift.
export { DotRegError, parse, parseEach, decodeString, appendValue, setText, setTextAt } from '../javascript/dotreg.mjs';
export type { Entry, Text, WriteValue, Source, ValueForm } from '../javascript/dotreg.mjs';
