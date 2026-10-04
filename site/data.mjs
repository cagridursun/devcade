export const games = ['snake', 'blockdrop', 'mazechase', 'blastgrid'];
export const gameNames = { snake: 'Snake', blockdrop: 'Block Drop', mazechase: 'Maze Chase', blastgrid: 'Blast Grid' };

// Keep the public website response small and omit identity fields it does not use.
export function cleanRows(game, rows) {
  if (!games.includes(game) || !Array.isArray(rows) || rows.length > 20) throw new Error('Invalid leaderboard');
  const names = new Set();
  let previousScore = Infinity;
  return rows.map((row, index) => {
    if (!row || row.rank !== index + 1 || typeof row.username !== 'string' || !/^[a-z0-9_]{3,20}$/.test(row.username) ||
      names.has(row.username) || !Number.isSafeInteger(row.score) || row.score < 0 || row.score > 1_000_000_000 ||
      row.score > previousScore || (game === 'snake' && (row.score > 6450 || row.score % 10 !== 0))) {
      throw new Error('Invalid leaderboard row');
    }
    names.add(row.username);
    previousScore = row.score;
    return { rank: row.rank, username: row.username, score: row.score };
  });
}

export function cleanSnapshot(value) {
  if (!value || value.version !== 1 || !value.boards || !Number.isFinite(Date.parse(value.generated_at))) throw new Error('Invalid snapshot');
  const boards = {};
  for (const game of games) {
    const board = value.boards[game];
    if (!board || !['ok', 'stale', 'unavailable'].includes(board.status)) throw new Error('Invalid board status');
    const rows = cleanRows(game, board.rows);
    if (board.status === 'unavailable') {
      if (board.updated_at !== null || rows.length) throw new Error('Invalid unavailable board');
    } else if (!Number.isFinite(Date.parse(board.updated_at))) throw new Error('Invalid update date');
    boards[game] = { status: board.status, updated_at: board.updated_at, rows };
  }
  return { version: 1, generated_at: value.generated_at, boards };
}

// A tab left open and an interrupted update must never appear freshly updated.
export function isStale(board, now = Date.now()) {
  return board.status === 'stale' || (board.updated_at !== null && now - Date.parse(board.updated_at) > 45 * 60 * 1000);
}
