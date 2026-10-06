import { cp, mkdir, rm, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { collectBoards, fetchJSON } from './snapshot.mjs';

const root = fileURLToPath(new URL('../../', import.meta.url));
const output = resolve(root, 'dist/pages');
const endpoint = 'https://devcade.cinesdigital.com';
const previousURL = process.env.PAGES_PREVIOUS_URL || 'https://cagridursun.github.io/devcade/leaderboards.json';
const offline = process.argv.includes('--offline');
let previous = null;
if (!offline) {
  try { previous = await fetchJSON(previousURL); } catch { /* Initial deployment or previous site unavailable. */ }
}
const fetcher = offline ? async () => { throw new Error('Offline validation'); } : fetch;
const snapshot = await collectBoards({ endpoint, previous, fetcher });
await rm(output, { recursive: true, force: true });
await mkdir(resolve(output, 'assets'), { recursive: true });
await mkdir(resolve(output, 'assets', 'games'), { recursive: true });
for (const file of ['index.html', 'style.css', 'app.mjs', 'data.mjs', 'favicon.svg', 'animation.css', 'usage.mjs', 'leaderboards.mjs']) await cp(resolve(root, 'site', file), resolve(output, file));
await cp(resolve(root, 'site/assets/devcade-showcase.gif'), resolve(output, 'assets', 'devcade-showcase.gif'));
for (const game of ['snake', 'blockdrop', 'mazechase', 'blastgrid', 'brickbreaker', 'terminalfc', 'spaceshooter']) {
  await cp(resolve(root, 'site', 'assets', 'games', `${game}.gif`), resolve(output, 'assets', 'games', `${game}.gif`));
}
await writeFile(resolve(output, 'leaderboards.json'), `${JSON.stringify(snapshot)}\n`);
await writeFile(resolve(output, '.nojekyll'), '');
for (const [game, board] of Object.entries(snapshot.boards)) console.log(`${game}: ${board.status}, ${board.rows.length} rows, updated ${board.updated_at || 'unavailable'}`);
console.log(`Website built at ${output}`);
