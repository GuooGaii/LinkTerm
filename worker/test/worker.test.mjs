import test from 'node:test';
import assert from 'node:assert/strict';
import { Miniflare, convertV4MiniflareOptions } from 'miniflare';
import { generateKeyPair, exportJWK, SignJWT } from 'jose';

test('real Worker validates Access and agent credentials; forwards bytes and rejects role forgery',async()=>{
 const { publicKey, privateKey }=await generateKeyPair('RS256');
 const jwk=await exportJWK(publicKey);jwk.kid='test';jwk.alg='RS256';jwk.use='sig';
 const mf=new Miniflare(convertV4MiniflareOptions({modules:true,scriptPath:'dist/worker.js',compatibilityDate:'2026-10-18',compatibilityFlags:['nodejs_compat'],durableObjects:{SESSIONS:{className:'SessionManager',useSQLite:true}},bindings:{AGENT_TOKEN:'test-secret',SESSION_ID:'test',ACCESS_TEAM_DOMAIN:'test.cloudflareaccess.com',ACCESS_AUD:'aud',ALLOWED_EMAIL:'friend@example.com'},outboundService:async request=>new Response(JSON.stringify({keys:[jwk]}),{headers:{'Content-Type':'application/json'}})}));
 const sockets=[];
 try {
  assert.equal((await mf.dispatchFetch('https://local/')).status,403);
  assert.equal((await mf.dispatchFetch('https://local/agent',{headers:{Upgrade:'websocket','X-LinkTerm-Role':'agent'}})).status,403);
  const token=await new SignJWT({email:'friend@example.com'}).setProtectedHeader({alg:'RS256',kid:'test'}).setIssuer('https://test.cloudflareaccess.com').setAudience('aud').setExpirationTime('1h').sign(privateKey);
  const headers={'Cf-Access-Jwt-Assertion':token,Origin:'https://local',Upgrade:'websocket'};
  assert.equal((await mf.dispatchFetch('https://local/',{headers})).status,200);
  assert.equal((await mf.dispatchFetch('https://local/ws',{headers:{...headers,Origin:'https://evil'}})).status,403);
  const agent=(await mf.dispatchFetch('https://local/agent',{headers:{Authorization:'Bearer test-secret',Upgrade:'websocket'}})).webSocket;
  assert.ok(agent);agent.accept();sockets.push(agent);
  const web=(await mf.dispatchFetch('https://local/ws',{headers:{...headers,'X-LinkTerm-Role':'agent'}})).webSocket;
  assert.ok(web);web.accept();sockets.push(web);
  const message = s => new Promise((resolve,reject)=>{const timer=setTimeout(()=>reject(new Error('timeout')),3000);s.addEventListener('message',e=>{clearTimeout(timer);resolve(JSON.parse(e.data))},{once:true});});
  let received=message(agent);
  web.send(JSON.stringify({type:'system',epoch:'first',data:{cmd:'attach',args:{cols:90,rows:30}}}));
  assert.equal((await received).data.args.cols,90);
  received=message(web);agent.send(JSON.stringify({type:'output',epoch:'first',data:'G1sySg=='}));assert.equal((await received).data,'G1sySg==');
  assert.equal((await mf.dispatchFetch('https://local/ws',{headers})).status,409);
  const closed=new Promise(resolve=>web.addEventListener('close',resolve,{once:true}));
  web.send(JSON.stringify({type:'output',epoch:'first',data:'YWJj'}));
  assert.equal((await closed).code,1008);
 }finally{for(const s of sockets)try{s.close()}catch{}await mf.dispose();}
});
