import { games, cleanRows, cleanSnapshot } from './data.mjs';

export async function fetchJSON(url, fetcher = fetch) {
  const response = await fetcher(url, { signal: AbortSignal.timeout(8000), headers: { Accept: 'application/json' }, redirect: 'error', cache: 'no-store', credentials: 'omit', referrerPolicy: 'no-referrer' });
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

// The published snapshot is a fallback, never a replacement for a newer live
// board already read in this tab. Each game fails independently.
export async function loadBoards({ endpoint, snapshotURL, previous, fetcher = fetch, now = new Date().toISOString() }) {
  let saved = null;
  try { saved = cleanSnapshot(previous); } catch { /* First load. */ }
  try {
    const published = cleanSnapshot(await fetchJSON(snapshotURL, fetcher));
    if (saved) {
      for (const game of games) {
        if (Date.parse(saved.boards[game].updated_at) > Date.parse(published.boards[game].updated_at || '1970-01-01')) {
          published.boards[game] = saved.boards[game];
        }
      }
    }
    saved = published;
  } catch { /* Keep the last valid in-memory snapshot. */ }
  return collectBoards({ endpoint, previous: saved, fetcher, now });
}
