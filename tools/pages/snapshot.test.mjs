import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { games, cleanRows, cleanSnapshot, isStale } from '../../site/data.mjs';
import { collectBoards, fetchJSON } from './snapshot.mjs';

const now = '2026-10-04T14:00:00.000Z';
const previousDate = '2026-10-04T13:45:00.000Z';
const row = { rank: 1, username: 'cagridursun', score: 310, player_id: 'public-but-unneeded' };
const response = (value) => ({ ok: true, text: async () => JSON.stringify(value) });
const old = () => ({ version: 1, generated_at: previousDate, boards: Object.fromEntries(games.map(game => [game, { status: 'ok', updated_at: previousDate, rows: [{ rank: 1, username: 'player_one', score: 20 }] }])) });

test('fetches all four independent boards and strips public player IDs', async () => {
  const requested = [];
  const data = await collectBoards({ endpoint: 'https://example.test', now, fetcher: async url => { requested.push(url); return response({ rows: [row] }); } });
  assert.equal(requested.length, 4);
  for (const game of games) {
    assert.ok(requested.includes(`https://example.test/v1/leaderboards/${game}`));
    assert.deepEqual(data.boards[game], { status: 'ok', updated_at: now, rows: [{ rank: 1, username: 'cagridursun', score: 310 }] });
  }
});

test('a real empty board stays distinguishable from an API failure', async () => {
  const data = await collectBoards({ endpoint: 'https://example.test', now, fetcher: async url => {
    if (url.endsWith('/snake')) return response({ rows: [] });
    throw new Error('Network unavailable');
  } });
  assert.deepEqual(data.boards.snake, { status: 'ok', updated_at: now, rows: [] });
  assert.deepEqual(data.boards.blockdrop, { status: 'unavailable', updated_at: null, rows: [] });
});

test('one failed game preserves its last successful rows and timestamp', async () => {
  const data = await collectBoards({ endpoint: 'https://example.test', previous: old(), now, fetcher: async url => {
    if (url.endsWith('/mazechase')) throw new Error('Unavailable');
    return response({ rows: [row] });
  } });
  assert.deepEqual(data.boards.mazechase, { ...old().boards.mazechase, status: 'stale' });
  assert.equal(data.boards.snake.updated_at, now);
  assert.equal(data.boards.snake.status, 'ok');
});

test('invalid previous data is discarded rather than published', async () => {
  const previous = old();
  previous.boards.snake.rows[0].username = '<img src=x onerror=alert(1)>';
  const data = await collectBoards({ endpoint: 'https://example.test', previous, now, fetcher: async () => { throw new Error('Offline'); } });
  for (const game of games) assert.equal(data.boards[game].status, 'unavailable');
});

test('malformed live data falls back without erasing other games', async () => {
  const data = await collectBoards({ endpoint: 'https://example.test', previous: old(), now, fetcher: async () => response({ rows: [{ ...row, rank: 99 }] }) });
  for (const game of games) assert.equal(data.boards[game].status, 'stale');
});

test('rejects untrusted names, invalid scores, ranks, duplicate aliases and unsorted results', () => {
  for (const patch of [{ username: '<script>' }, { username: 123 }, { score: -1 }, { score: 1.5 }, { score: 1_000_000_001 }, { rank: 0 }]) {
    assert.throws(() => cleanRows('blockdrop', [{ ...row, ...patch }]));
  }
  assert.throws(() => cleanRows('snake', [{ ...row, score: 315 }]));
  assert.throws(() => cleanRows('snake', [{ ...row, score: 6460 }]));
  assert.throws(() => cleanRows('snake', [row, { ...row, rank: 2 }]));
  assert.throws(() => cleanRows('snake', [row, { ...row, rank: 2, username: 'player_two', score: 400 }]));
  assert.throws(() => cleanRows('unknown', []));
  assert.throws(() => cleanRows('snake', Array(21).fill(row)));
});

test('HTTP errors, redirects and oversized responses cannot become a board', async () => {
  await assert.rejects(() => fetchJSON('https://example.test', async () => ({ ok: false, status: 500 })));
  await assert.rejects(() => fetchJSON('https://example.test', async () => ({ ok: true, text: async () => 'x'.repeat(32769) })));
  await fetchJSON('https://example.test', async (_url, options) => { assert.equal(options.redirect, 'error'); assert.ok(options.signal); return response({ rows: [] }); });
});

test('outdated scores are flagged even if the scheduled workflow stops', () => {
  assert.equal(isStale(old().boards.snake, Date.parse(now)), false);
  assert.equal(isStale(old().boards.snake, Date.parse(now) + 31 * 60 * 1000), true);
  assert.equal(isStale({ ...old().boards.snake, status: 'stale' }, Date.parse(now)), true);
});

test('snapshot schema rejects missing boards and inconsistent availability', () => {
  assert.throws(() => cleanSnapshot({ ...old(), version: 2 }));
  const data = old(); delete data.boards.blastgrid;
  assert.throws(() => cleanSnapshot(data));
  const bad = old(); bad.boards.snake.status = 'unavailable';
  assert.throws(() => cleanSnapshot(bad));
});

test('site has working install commands, both languages, social links and safe text rendering', async () => {
  const html = await readFile(new URL('../../site/index.html', import.meta.url), 'utf8');
  const script = await readFile(new URL('../../site/app.mjs', import.meta.url), 'utf8');
  assert.ok(html.includes('brew install cagridursun/devcade/devcade'));
  assert.ok(html.includes('scoop bucket add devcade https://github.com/cagridursun/scoop-devcade'));
  assert.ok(html.includes('scoop install devcade'));
  assert.ok(html.includes('https://x.com/c__dursun'));
  assert.ok(html.includes('data-language="en"') && html.includes('data-language="tr"'));
  assert.ok(script.includes("td.textContent = value"));
  assert.ok(!script.includes('innerHTML'));
  const dictionary = script.slice(script.indexOf('const messages = ') + 17, script.indexOf('\n\nlet language'));
  const messages = Function(`return (${dictionary.replace(/;\s*$/, '')})`)();
  assert.deepEqual(Object.keys(messages.en).sort(), Object.keys(messages.tr).sort());
  for (const key of [...html.matchAll(/data-i18n(?:-alt|-aria)?="([^"]+)"/g)].map(m => m[1])) {
    assert.ok(messages.en[key], `Missing English: ${key}`);
    assert.ok(messages.tr[key], `Missing Turkish: ${key}`);
  }
});
