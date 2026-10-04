import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { cleanReplay, frameAt } from '../../site/snake-preview.mjs';

const source = JSON.parse(await readFile(new URL('../../site/assets/snake-demo.json', import.meta.url), 'utf8'));
const replay = cleanReplay(source);

test('recording is a full 80x24 game clip with movement, eating, growth and no game over', () => {
  assert.ok(replay.duration >= 24000 && replay.duration < 25000);
  const hud = replay.frames.map(frame => frame.text[0]);
  assert.ok(hud[0].includes('Score 0'));
  assert.ok(hud.some(line => /Score [4-9]\d/.test(line)));
  assert.ok(replay.frames.every(frame => !frame.text.join('\n').includes('GAME OVER')));
  assert.ok(new Set(replay.frames.map(frame => frame.text.join('\n'))).size > 100);
  for (const frame of replay.frames) {
    const head = frame.text.join('').match(/@@/g);
    assert.equal(head?.length, 1);
    const food = frame.text.join('').match(/\*\*/g);
    assert.equal(food?.length, 1);
  }
});

test('replay respects actual frame durations and loops at its boundary', () => {
  assert.equal(frameAt(replay, 0), 0);
  assert.equal(frameAt(replay, replay.frames[0].duration - 1), 0);
  assert.equal(frameAt(replay, replay.frames[0].duration), 1);
  assert.equal(frameAt(replay, replay.duration - 1), replay.frames.length - 1);
  assert.equal(frameAt(replay, replay.duration), 0);
  assert.equal(frameAt(replay, replay.duration * 3 + replay.frames[0].duration), 1);
});

test('malformed playback data cannot replace the static fallback', () => {
  for (const patch of [{ cols: 79 }, { version: 2 }, { frames: [] }]) assert.throws(() => cleanReplay({ ...source, ...patch }));
  const bad = structuredClone(source);
  bad.frames[0].duration = 0;
  assert.throws(() => cleanReplay(bad));
  bad.frames[0].duration = 180;
  bad.frames[0].colors[0] = '9'.repeat(80);
  assert.throws(() => cleanReplay(bad));
});

test('hero has a poster, canvas, localized controls and built playback assets', async () => {
  const html = await readFile(new URL('../../site/index.html', import.meta.url), 'utf8');
  const app = await readFile(new URL('../../site/app.mjs', import.meta.url), 'utf8');
  const build = await readFile(new URL('./build.mjs', import.meta.url), 'utf8');
  assert.ok(html.includes('id="snake-poster"') && html.includes('id="snake-preview"') && html.includes('id="animation-toggle"'));
  assert.ok(app.includes('pauseAnimation') && app.includes('Animasyonu duraklat'));
  assert.ok(build.includes('snake-preview.mjs') && build.includes('snake-demo.json'));
});
