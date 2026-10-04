import test from 'node:test';
import assert from 'node:assert/strict';
import { initUsage } from '../../site/usage.mjs';

test('website makes no measurement request before consent and cancels pending requests on opt-out', () => {
  const originals = Object.fromEntries(['document','localStorage','sessionStorage','fetch'].map(k=>[k,globalThis[k]]));
  const values=new Map();const calls=[];let click;
  const storage={getItem:k=>values.get(k)||null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)};
  const button={textContent:'',addEventListener:(_,handler)=>{click=handler;},setAttribute:()=>{}};
  globalThis.document={querySelector:()=>button};globalThis.localStorage=storage;globalThis.sessionStorage=storage;
  globalThis.fetch=(_,options)=>{calls.push(options);return new Promise((resolve,reject)=>options.signal.addEventListener('abort',()=>reject(new Error('aborted'))));};
  try {
    const usage=initUsage();usage.track('install_copy');
    assert.equal(calls.length,0);assert.equal(values.size,0);
    click();assert.equal(calls.length,1);assert.equal(JSON.parse(calls[0].body).kind,'site_visit');
    usage.track('install_copy');assert.equal(calls.length,2);
    const payload=JSON.parse(calls[1].body);assert.match(payload.installation,/^[0-9a-f]{32}$/);assert.equal(payload.platform,'browser');assert.equal(Object.hasOwn(payload,'username'),false);
    click();assert.equal(calls.every(c=>c.signal.aborted),true);assert.equal(values.get('devcade-site-usage'),'off');assert.equal(values.has('devcade-site-usage-id'),false);
    usage.track('install_copy');assert.equal(calls.length,2);
  } finally {for(const [key,value] of Object.entries(originals)){if(value===undefined)delete globalThis[key];else globalThis[key]=value;}}
});
