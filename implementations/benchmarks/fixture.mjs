// The same byte-for-byte fixture is consumed by every native API benchmark.
import { writeFile } from 'node:fs/promises';
const [filename, countArg = '20000'] = process.argv.slice(2);
const count = Number(countArg);
if (!filename || !Number.isSafeInteger(count) || count < 1) throw new Error('Usage: node fixture.mjs output [positive_setting_count]');
const lines = ['@dotReg 1', ''];
for (let i = 0; i < count; i++) {
  if (i % 256 === 0) lines.push(`Group${Math.floor(i / 256)}.`);
  switch (i % 8) {
    case 0: lines.push('  .Tag:001'); break;
    case 1: lines.push('  .Tag(string):"hello\\tworld \\u{1F600}"'); break;
    case 2: lines.push('  .Path:C:\\temp\\new'); break;
    case 3: lines.push('  ."名称.B":日本語'); break;
    case 4: lines.push('  .Unset:'); break;
    case 5: lines.push('  .Literal:', '   | literal \\q "quotes"', '   ||second line'); break;
    case 6: lines.push('  .Message:"""', '   first\\nline', '     second', '   """'); break;
    case 7: lines.push('  .Long:' + 'x'.repeat(256)); break;
  }
}
await writeFile(filename, lines.join('\n') + '\n');
