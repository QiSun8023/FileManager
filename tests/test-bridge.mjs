import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
const source=await readFile(new URL('../module/webroot/bridge.js',import.meta.url),'utf8');
const { createBackend }=await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
const result=data=>JSON.stringify({ok:true,data});
let calls=[];
const backend=createBackend(async command=>{calls.push(command);if(command.includes(' info '))return result({version:'v1.7.2'});return result({phase:'idle'});},'v1.7.2');
assert.deepEqual(await backend('status'),{phase:'idle'});
assert.equal(calls.length,2);
assert.match(calls[0],/ info 'e30='/);
assert.match(calls[1],/ status 'e30='/);
console.log('PASS checks backend version before execution');

calls=[];
const wrong=createBackend(async command=>{calls.push(command);return result({version:'v1.1.0'});},'v1.7.2');
await assert.rejects(()=>wrong('clean',{ids:['one']}),/版本不一致/);
assert.equal(calls.length,1);
assert.ok(!calls.some(c=>c.includes(' clean ')));
console.log('PASS stale page/backend mismatch blocks mutations');

calls=[];
await assert.rejects(()=>backend('unknown; rm -rf /'),/未知操作/);
assert.equal(calls.length,0);
console.log('PASS rejects unknown actions before shell execution');

const payload={root:"/storage/emulated/0/中文 ' ` $(touch bad)\n文件"};
await backend('start',payload);
const actual=calls.at(-1);
assert.ok(!actual.includes('touch'));
const encoded=actual.match(/start '([^']+)'$/)[1];
assert.deepEqual(JSON.parse(Buffer.from(encoded,'base64').toString('utf8')),payload);
console.log('PASS Unicode and shell-sensitive paths stay inside Base64 JSON');

const invalid=createBackend(async()=> 'not json','v1.7.2');
await assert.rejects(()=>invalid('status'),/后端响应无效/);
const error=createBackend(async()=>JSON.stringify({ok:false,error:'no access'}),'v1.7.2');
await assert.rejects(()=>error('status'),/no access/);
console.log('PASS reports invalid responses and backend errors');
console.log('Bridge: 5 scenarios passed.');
