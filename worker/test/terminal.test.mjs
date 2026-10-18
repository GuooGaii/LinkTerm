import test from 'node:test';
import assert from 'node:assert/strict';
import headless from '@xterm/headless';
import { decodeOutput } from '../src/terminal-stream.js';
import { validMessage } from '../src/protocol.js';
const { Terminal } = headless;
const write = (t,b) => new Promise(resolve => t.write(b,resolve));
const screen = t => Array.from({length:t.rows},(_,i)=>t.buffer.active.getLine(t.buffer.active.viewportY+i).translateToString(true)).join('\n');
test('100 in-place AI UI redraws retain one final frame, even across byte boundaries', async()=>{
 const t=new Terminal({cols:40,rows:12,allowProposedApi:true});
 await write(t,'\x1b[?1049h');
 for(let i=0;i<100;i++){
  const bytes=Buffer.from(`\x1b[H\x1b[2K任务 ${i}\r\n\x1b[2K正在处理 🐈\r\n\x1b[2K> 输入`);
  for(let n=0;n<bytes.length;n+=3) await write(t,decodeOutput(bytes.subarray(n,n+3).toString('base64')));
 }
 assert.equal((screen(t).match(/任务/g)||[]).length,1);
 assert.match(screen(t),/任务 99/); assert.match(screen(t),/正在处理 🐈/); assert.doesNotMatch(screen(t),/�/);
 assert.equal(t.buffer.active.baseY,0);t.dispose();
});
test('cursor-up / carriage return / erase update normal screen rather than append',async()=>{
 const t=new Terminal({cols:80,rows:24,allowProposedApi:true});
 await write(t,'Header\r\nProgress 0\r\nPrompt');
 for(let i=1;i<=100;i++) await write(t,`\x1b[2A\r\x1b[2KHeader\r\n\x1b[2KProgress ${i}\r\n\x1b[2KPrompt`);
 assert.equal((screen(t).match(/Header/g)||[]).length,1); assert.match(screen(t),/Progress 100/);assert.equal(t.buffer.active.baseY,0);
 t.resize(50,20);await write(t,'\x1b[H\x1b[2JResized');assert.match(screen(t),/^Resized/);t.dispose();
});
test('clients cannot forge agent output, register roles, or send unbounded geometry',()=>{
 assert.equal(validMessage('web',{type:'output',epoch:'a',data:'YWJj'}),false);
 assert.equal(validMessage('web',{type:'register',role:'agent'}),false);
 assert.equal(validMessage('web',{type:'system',epoch:'a',data:{cmd:'resize',args:{cols:50000,rows:24}}}),false);
 assert.equal(validMessage('agent',{type:'input',epoch:'a',data:'rm'}),false);
 assert.equal(validMessage('web',{type:'system',epoch:'a',data:{cmd:'attach',args:{cols:80,rows:24,allowProposedApi:true}}}),true);
});
