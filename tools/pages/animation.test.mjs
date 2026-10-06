import test from 'node:test';
import assert from 'node:assert/strict';
import { access, readFile } from 'node:fs/promises';

const games = ['snake', 'blockdrop', 'mazechase', 'blastgrid', 'brickbreaker', 'terminalfc', 'spaceshooter'];

test('hero uses the real seven-game gameplay GIF', async () => {
  const html = await readFile(new URL('../../site/index.html', import.meta.url), 'utf8');
  const build = await readFile(new URL('./build.mjs', import.meta.url), 'utf8');

  assert.ok(html.includes('src="./assets/devcade-showcase.gif"'));
  assert.ok(html.includes('devcade / seven games'));
  assert.ok(!html.includes('id="snake-preview"'));
  assert.ok(!html.includes('id="animation-toggle"'));
  assert.ok(build.includes('site/assets/devcade-showcase.gif'));

  await access(new URL('../../site/assets/devcade-showcase.gif', import.meta.url));
});

test('all seven game cards use real gameplay GIFs', async () => {
  const html = await readFile(new URL('../../site/index.html', import.meta.url), 'utf8');
  const build = await readFile(new URL('./build.mjs', import.meta.url), 'utf8');

  for (const game of games) {
    assert.ok(html.includes(`src="./assets/games/${game}.gif"`), `missing ${game} gameplay GIF in site/index.html`);
    assert.ok(build.includes(`${game}.gif`) || build.includes("${game}.gif"), `build does not publish ${game}.gif`);
    await access(new URL(`../../site/assets/games/${game}.gif`, import.meta.url));
  }
});

test('README hero uses the real gameplay reel', async () => {
  const readme = await readFile(new URL('../../README.md', import.meta.url), 'utf8');
  assert.ok(readme.includes('docs/assets/devcade-seven-games.gif'));
  await access(new URL('../../docs/assets/devcade-seven-games.gif', import.meta.url));
});
