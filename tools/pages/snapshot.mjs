import { games, cleanRows, cleanSnapshot } from '../../site/data.mjs';

export async function fetchJSON(url, fetcher = fetch) {
  const response = await fetcher(url, { signal: AbortSignal.timeout(8000), headers: { Accept: 'application/json' }, redirect: 'error' });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  const text = await response.text();
  if (text.length > 32768) throw new Error('Response too large');
  return JSON.parse(text);
}

export async function collectBoards({ endpoint, previous, fetcher = fetch, now = new Date().toISOString() }) {
  let saved = null;
  try { saved = cleanSnapshot(previous); } catch { /* No usable previous deployment. */ }
  const boards = {};
  await Promise.all(games.map(async (game) => {
    try {
      const response = await fetchJSON(`${endpoint}/v1/leaderboards/${game}`, fetcher);
      boards[game] = { status: 'ok', updated_at: now, rows: cleanRows(game, response.rows) };
    } catch {
      const old = saved?.boards[game];
      boards[game] = old && old.status !== 'unavailable'
        ? { ...old, status: 'stale' }
        : { status: 'unavailable', updated_at: null, rows: [] };
    }
  }));
  return cleanSnapshot({ version: 1, generated_at: now, boards });
}
