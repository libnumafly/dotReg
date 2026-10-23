import { parseEach, decodeString, type Entry } from './dotreg.mjs';

const settings: string[] = [];
await parseEach('@dotReg 1\n\nA.Code:001\nA.Code:002\n', (entry: Entry) => {
  if (entry.kind === 'value') settings.push(decodeString(entry).value);
});
if (settings.join(',') !== '001,002') throw new Error('unexpected settings');
