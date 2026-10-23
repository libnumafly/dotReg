// Hand-specified expectations, independent of any parser. Regenerate cases.json.
import { writeFile } from 'node:fs/promises';
const cases = [];
const prefix = '@dotReg 1\n\n';
const v = (path, raw, decoded = raw, extra = {}) => ({
  line: 3, kind: 'value', path: Array.isArray(path) ? path : [path], raw,
  unset: raw === '', has_type: false, decoded, ...extra,
});
const h = (path, line = 3) => ({ line, kind: 'heading', path, raw: '', unset: false, has_type: false });
function ok(name, body, entries) { cases.push({ name, input: prefix + body, entries }); }
function bad(name, body, line = 3, code = 'syntax') {
  cases.push({ name, input: prefix + body, error: { stage: 'parse', line, code } });
}
function badFile(name, input, line, code = 'syntax') { cases.push({ name, input, error: { stage: 'parse', line, code } }); }
function badString(name, raw) {
  const entry = v('A', raw); delete entry.decoded;
  cases.push({ name, input: prefix + 'A:' + raw, entries: [entry], error: { stage: 'decode', line: 3, code: 'string' } });
}
ok('empty body', '', []);
cases.push({ name: 'BOM CRLF declaration and tab separator', input: '\ufeff@dotReg\t1 \t\r\n\t \r\nA:001', entries: [v('A', '001')] });
cases.push({ name: 'final whitespace separator without newline', input: '@dotReg 1\n \t', entries: [] });
for (const [name, input, line, code] of [
  ['empty file', '', 1], ['declaration only', '@dotReg 1', 2],
  ['no physical separator', '@dotReg 1\n', 2], ['no declaration', 'A:x\n', 1],
  ['leading blank', '\n@dotReg 1\n\n', 1], ['property comment', '# secret\n@dotReg 1\n\n', 1],
  ['indented declaration', ' @dotReg 1\n\n', 1], ['missing property value', '@dotReg \n\n', 1],
  ['unsupported version', '@dotReg 2\n\n', 1, 'unsupported'],
  ['unknown property', '@Other 1\n\n', 1, 'unsupported'],
  ['duplicate declaration', '@dotReg 1\n@dotReg 1\n\n', 2, 'duplicate'],
  ['quoted version', '@dotReg "1"\n\n', 1, 'unsupported'],
  ['property inline comment', '@dotReg 1 #comment\n\n', 1, 'unsupported'],
  ['second BOM', '\ufeff\ufeff@dotReg 1\n\n', 1],
]) badFile(name, input, line, code);
for (const raw of ['001', 'true', 'null', 'a:b:#c', 'say "hello', 'say """ \\q', 'C:\\temp\\new', '\\q', 'end\\', '(a(b))', '|', '\u00a0']) {
  ok(`literal ${JSON.stringify(raw)}`, 'A:' + raw, [v('A', raw)]);
}
ok('ASCII outer trim only', 'A: \t a  b\u00a0 \t', [v('A', 'a  b\u00a0')]);
ok('unset', 'A: \t', [v('A', '')]);
ok('explicit empty quote', 'A:""', [v('A', '""', '', { unset: false })]);
ok('quoted whitespace retained', 'A: "  value \\t " ', [v('A', '"  value \\t "', '  value \t ')]);
const escapes = [['\\.', '.'], ['\\"', '"'], ['\\\\', '\\'], ['\\/', '/'], ['\\b', '\b'], ['\\f', '\f'], ['\\n', '\n'], ['\\r', '\r'], ['\\t', '\t'], ['\\u0041', 'A'], ['\\u{41}', 'A'], ['\\u{000041}', 'A'], ['\\uD83D\\uDE00', '😀'], ['\\u{1f600}', '😀'], ['\\u{0}', '\0'], ['\\u{10FFFF}', '\u{10ffff}'], ['\\u{FFFF}', '\uffff'], ['\\u{0378}', '\u0378']];
for (const [escaped, decoded] of escapes) {
  ok(`value escape ${escaped}`, `A:"${escaped}"`, [v('A', `"${escaped}"`, decoded)]);
  ok(`key escape ${escaped}`, `"${escaped}":x`, [v(decoded, 'x')]);
}
for (const [key, path] of [['A.B', ['A','B']], ['"A.B"', ['A.B']], ['A."".B', ['A','','B']], ['""', ['']], ['"A:B()#"', ['A:B()#']], ['"A\tB"', ['A\tB']], ['"A\u0000B"', ['A\0B']], ['店舗.名称', ['店舗','名称']], ['01.1A.A_B-2', ['01','1A','A_B-2']]]) {
  ok(`path ${JSON.stringify(key)}`, key + ':001', [v(path, '001')]);
}
ok('parent and descendants coexist', 'A:r\nA.B:p\nA.B.C:c', [v('A','r'),v(['A','B'],'p','p',{line:4}),v(['A','B','C'],'c','c',{line:5})]);
ok('no Unicode normalisation', 'が:a\nが:b', [v('が','a'),v('が','b','b',{line:4})]);
ok('headings self repeated and dedent', 'A.\n  .B.\n    .:one\n    .C:x\n  .D:y\n  .D:y', [h(['A']),h(['A','B'],4),v(['A','B'],'one','one',{line:5}),v(['A','B','C'],'x','x',{line:6}),v(['A','D'],'y','y',{line:7}),v(['A','D'],'y','y',{line:8})]);
ok('dotted heading is one written level', 'A.B.\n  .C:x', [h(['A','B']),v(['A','B','C'],'x','x',{line:4})]);
ok('quoted heading and empty child', '"A.B".\n  ."":x\n  .:y', [h(['A.B']),v(['A.B',''],'x','x',{line:4}),v(['A.B'],'y','y',{line:5})]);
ok('comments blank indentation exempt', 'A.\n \t # comment\n\t\n  .B:x', [h(['A']),v(['A','B'],'x','x',{line:6})]);
ok('ordinary property name in body', '@Name: value', [v('@Name','value')]);
ok('opaque compound annotation', 'A (map(string,list(int))) : 001', [v('A','001','001',{type:'map(string,list(int))',has_type:true})]);
ok('repeats retain different annotations and unset', 'A(int):1\nA:\nA(string):""', [v('A','1','1',{type:'int',has_type:true}),v('A','','',{line:4}),v('A','""','',{line:5,type:'string',has_type:true,unset:false})]);
ok('empty triple', 'A:"""\n"""', [v('A','','',{unset:false,form:'triple_quoted'})]);
ok('triple margin and escapes', 'A:"""\n   one\\n\n     two\n   """ \t', [v('A','one\\n\n  two','one\n\n  two',{unset:false,form:'triple_quoted'})]);
ok('triple empty physical line exempt', 'A:"""\n\n  x\n  """', [v('A','\nx','\nx',{unset:false,form:'triple_quoted'})]);
ok('triple trailing newline', 'A:"""\n x\n\n """', [v('A','x\n','x\n',{unset:false,form:'triple_quoted'})]);
ok('triple literal quotes', 'A:"""\nsay "hello" and """data\n"""', [v('A','say "hello" and """data','say "hello" and """data',{unset:false,form:'triple_quoted'})]);
ok('triple declaration is data', 'A:"""\n@dotReg 99\n"""', [v('A','@dotReg 99','@dotReg 99',{unset:false,form:'triple_quoted'})]);
cases.push({name:'mixed block EOL retained',input:prefix+'A:"""\r\none\r\ntwo\nthree\r\n"""',entries:[v('A','one\r\ntwo\nthree','one\r\ntwo\nthree',{unset:false,form:'triple_quoted'})]});
ok('pipe literal varied odd margins', 'A:\n | C:\\new\\q \t\n   ||two\nB:x', [v('A',' C:\\new\\q \t\n|two',' C:\\new\\q \t\n|two',{unset:false,form:'pipe'}),v('B','x','x',{line:6})]);
ok('empty pipe', 'A:\n |', [v('A','','',{unset:false,form:'pipe'})]);
ok('pipe trailing newline', 'A:\n |x\n |', [v('A','x\n','x\n',{unset:false,form:'pipe'})]);
ok('pipe ends before comment', 'A:\n |x\n#end\nB:y', [v('A','x','x',{unset:false,form:'pipe'}),v('B','y','y',{line:6})]);
ok('same level pipe is ordinary key', 'A:\n|:x', [v('A',''),v('|','x','x',{line:4})]);
ok('relative typed self pipe', 'A.\n  .(text):\n   |x\n  .B:y', [h(['A']),v('A','x','x',{line:4,form:'pipe',unset:false,has_type:true,type:'text'}),v(['A','B'],'y','y',{line:6})]);
for (const key of ['', 'A..B', 'A.', 'A\\.B', 'A#B', 'A B', 'A\tB', 'A\u0000B', 'A\u00a0B', 'A\u0085B', 'A\u3000B', 'A)B', 'A"B"', '"A"B', '"A', 'A."B"C']) bad(`invalid key ${JSON.stringify(key)}`, key + ':SECRET');
for (const [name, body, line] of [
  ['relative top level','.A:x',3],['indented absolute','  A:x',3],['odd indent',' A:x',3],
  ['tab indent','\t.A:x',3],['missing heading','  .A:x',3],['skipped level','A.\n    .B:x',4],
  ['absolute closes headings','A.\nB:x\n  .C:x',5],['value dedent closes heading','A.\n  .B.\n  .C:x\n    .D:x',6],
  ['missing trailing heading element','A..',3],['body declaration','@dotReg 1',3],
  ['unclosed type','A(list(int):x',3],['internal type space','A(list( int)):x',3],
  ['heading annotation','A(int).',3],['type tail','A(int)other:x',3],['lone CR','A:secret\rdata',3],
  ['triple missing close','A:"""\nx',3],['triple insufficient margin','A:"""\n x\n  """',4],
  ['no pipe across blank','A:\n\n |x',5],['no pipe across comment','A:\n#comment\n |x',5],
  ['pipe stops at blank','A:\n |x\n\n |y',6],['indented absolute pipe','A.\n  .B:\n  |:x',5],
]) bad(name,body,line);
for (const [name, body] of [['empty type','A():x'],['type quote','A("x"):y'],['type backslash','A(x\\y):z'],['tab before type','A\t(int):1'],['tab after type','A(int)\t:1']]) bad(name,body,3,'unsupported');
const invalidEscapes = ['\\q','\\u{}','\\u{1234567}','\\u{110000}','\\u{D800}','\\u{xyz}','\\u{1_2}','\\u{12','\\u12','\\uD800','\\uDC00','\\uDC00\\uD800','\\uD800\\u{DC00}','\\u{D800}\\uDC00','\\uD800\\u0041'];
for (const escaped of invalidEscapes) {
  badString(`invalid value escape ${escaped}`, `"${escaped}"`);
  bad(`invalid key escape ${escaped}`, `"${escaped}":SECRET`);
}
for (const raw of ['"unfinished','"ok"trailing','"""inline','"ends\\']) badString(`invalid quoted value ${JSON.stringify(raw)}`,raw);
const invalidTriple = v('A','\\q','',{form:'triple_quoted',unset:false}); delete invalidTriple.decoded;
cases.push({name:'triple decode failure after structural success',input:prefix+'A:"""\n\\q\n"""',entries:[invalidTriple],error:{stage:'decode',line:3,code:'string'}});
for (const hex of ['ff','c080','eda080','f4908080','e282']) cases.push({name:`invalid UTF8 ${hex}`,input_hex:Buffer.from(prefix+'A:').toString('hex')+hex,error:{stage:'parse',line:3,code:'syntax'}});
cases.push({name:'invalid UTF8 in qualifying pipe body',input_hex:Buffer.from(prefix+'A:\n |').toString('hex')+'ff',error:{stage:'parse',line:4,code:'syntax'}});
bad('collecting CLI validates syntax before decoding', 'A:"\\q"\ninvalid', 4);
for (const [name, body, line] of [['comment','#',3],['triple','A:"""\n',4],['declaration','@dotReg ',1]]) {
  const source = name === 'declaration' ? body : prefix + body;
  cases.push({name:`invalid UTF8 in ${name}`,input_hex:Buffer.from(source).toString('hex')+'ff',error:{stage:'parse',line,code:'syntax'}});
}
ok('BOM outside initial declaration is data', '\ufeffA:\ufeffx', [v('\ufeffA','\ufeffx')]);
bad('Unicode type whitespace', 'A(list(\u0085int)):x');
await writeFile(new URL('cases.json',import.meta.url), JSON.stringify({version:1,cases},null,2)+'\n');
console.log(`Wrote ${cases.length} independently specified cases`);
