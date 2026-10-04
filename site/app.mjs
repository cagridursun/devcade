import { loadBoards } from './leaderboards.mjs';
import { gameNames, isStale } from './data.mjs';
import { initSnakePreview } from './snake-preview.mjs';
import { initUsage } from './usage.mjs';

const messages = {
  en: {
    skip: 'Skip to content', nav: 'Main navigation', language: 'Page language', games: 'Games', install: 'Install', leaderboard: 'Leaderboard',
    usageOn: 'Site statistics: on', usageOff: 'Site statistics: off', usageNote: 'Optional site statistics count visits and successful command copies using a random session ID. No account or page URL is sent. Off by default.',
    eyebrow: 'A small break between builds', headline: ['Your terminal.', 'Your next high score.'],
    intro: 'Waiting on a build, a test run or an AI response? Open another terminal and play. DevCade brings four arcade games to the place you already work.',
    playAnimation: 'Play animation', pauseAnimation: 'Pause animation', animationAlt: 'Animated replay of actual Snake gameplay',
    get: 'Install DevCade', github: 'View on GitHub', open: 'Free & open source', realframe: 'Actual gameplay. Right in your terminal.',
    four: 'arcade games', five: 'in-game languages', three: 'colour palettes', offline: 'offline play', collection: 'The collection', pick: 'Pick your next break.', coming: 'New games coming soon.',
    snake: 'Eat, grow, turn. Try to beat your best without running into yourself.',
    block: 'Rotate falling pieces, clear rows and find room for just one more block.',
    maze: 'Clear the maze, dodge four chasers and turn the tables with a power pellet.',
    blast: 'Place bombs, break crates and outlast three bots. Leave yourself an escape route.',
    arrows: 'Arrows / WASD', rotate: 'Move / rotate / drop', bomb: 'Move / Z to bomb',
    snakeAlt: 'Snake running inside an 80 by 24 terminal', blockAlt: 'Block Drop with a falling piece and next-piece preview', mazeAlt: 'Maze Chase with pellets and four chasers', blastAlt: 'Blast Grid with bombs, crates and three bots',
    start: 'Ready when you are', onecommand: 'A couple of commands. Then play.', guide: 'Full installation guide', nogo: 'Ready-to-play binaries. No Go installation, source checkout or build required.',
    copy: 'Copy', copied: 'Copied!', copyFail: 'Could not copy. Select and copy the commands.', brewsetup: 'First time using Homebrew?', scoopsetup: 'First time using Scoop?', setup: 'Set it up here ↗',
    version: 'Current release: v1.0.0-rc.2', size: 'An interactive terminal of at least 80 × 24.', scores: 'The community scoreboard', highscores: 'A little friendly competition.', refresh: 'Refresh',
    leaderintro: "Each player's personal best, ranked separately for every game. Pick a game to see the top 20.", boardgame: 'Leaderboard game', loading: 'Loading scores…', rank: 'Rank', player: 'Player', score: 'Best score',
    periodic: 'Scores load when the page opens or you press Refresh. If the service is unavailable, the last available copy is shown.', personal: 'Personal bests per game', share: "Want to join the board? Choose a username and turn on global score sharing in DevCade's Settings. Sharing is off by default.",
    empty: 'No scores yet. Set the first high score!', error: 'Scores are temporarily unavailable. Please try Refresh in a moment.', stale: 'Live scores could not be refreshed. Showing the last available scores.', updated: 'Updated',
    built: 'Built by cagridursun', creator: 'Small games. Good breaks.', creatorcopy: "I'm Çağrı Dursun. I built DevCade for the little pauses in a developer's day. Follow along for new games and project updates.",
    follow: 'Follow me on X / Twitter', footer: 'Made for developers. MIT licensed.', feedback: 'Feedback & ideas ↗',
  },
  tr: {
    skip: 'İçeriğe geç', nav: 'Ana gezinme', language: 'Sayfa dili', games: 'Oyunlar', install: 'Kurulum', leaderboard: 'Skor tablosu',
    usageOn: 'Site istatistikleri: açık', usageOff: 'Site istatistikleri: kapalı', usageNote: 'İsteğe bağlı site ölçümü, rastgele oturum kimliğiyle ziyaretleri ve başarılı komut kopyalamalarını sayar. Hesap veya sayfa adresi gönderilmez. Varsayılan olarak kapalıdır.',
    eyebrow: "Build'ler arasında küçük bir mola", headline: ['Terminalin açık.', 'Sıradaki rekor senin.'],
    intro: 'Build, test ya da AI cevabı mı bekliyorsun? Başka bir terminal açıp oyna. DevCade, dört arcade oyununu zaten çalıştığın yere getiriyor.',
    playAnimation: 'Animasyonu oynat', pauseAnimation: 'Animasyonu duraklat', animationAlt: 'Gerçek Snake oyunundan animasyonlu tekrar',
    get: "DevCade'i yükle", github: "GitHub'da incele", open: 'Ücretsiz ve açık kaynak', realframe: 'Gerçek oyun görüntüsü. Doğrudan terminalinde.',
    four: 'arcade oyunu', five: 'oyun içi dil', three: 'renk paleti', offline: 'çevrimdışı oyun', collection: 'Oyun koleksiyonu', pick: 'Molanda ne oynayacaksın?', coming: 'Yeni oyunlar yakında.',
    snake: 'Ye, büyü, dön. Kendine çarpmadan kendi rekorunu geçmeye çalış.',
    block: 'Düşen parçaları döndür, satırları temizle ve bir parçaya daha yer aç.',
    maze: 'Labirenti temizle, dört takipçiden kaç ve güç pelletini alıp avantajı yakala.',
    blast: 'Bombaları yerleştir, kasaları kır ve üç bottan daha uzun dayan. Kaçış yolunu açık tut.',
    arrows: 'Yön tuşları / WASD', rotate: 'Hareket / döndür / düşür', bomb: 'Hareket / Z ile bomba',
    snakeAlt: '80 × 24 terminalde çalışan Snake oyunu', blockAlt: 'Düşen parçayı ve sıradaki parçayı gösteren Block Drop', mazeAlt: 'Pelletler ve dört takipçiyle Maze Chase', blastAlt: 'Bombalar, kasalar ve üç botla Blast Grid',
    start: 'Hazırsan başlayalım', onecommand: 'Birkaç komutla yükle, sonra oyna.', guide: 'Ayrıntılı kurulum rehberi', nogo: 'Oynamaya hazır paketler. Go yüklemene, kaynak kodu indirmene veya derlemene gerek yok.',
    copy: 'Kopyala', copied: 'Kopyalandı!', copyFail: 'Kopyalanamadı. Komutları seçip kopyalayabilirsin.', brewsetup: 'Homebrew ilk kez mi kullanıyorsun?', scoopsetup: 'Scoop ilk kez mi kullanıyorsun?', setup: 'Buradan kurabilirsin ↗',
    version: 'Güncel sürüm: v1.0.0-rc.2', size: 'En az 80 × 24 boyutunda etkileşimli bir terminal.', scores: 'Topluluğun skor tablosu', highscores: 'Biraz tatlı rekabet.', refresh: 'Yenile',
    leaderintro: 'Her oyuncunun en yüksek skoru, her oyun için ayrı bir tabloda. İlk 20 oyuncuyu görmek için oyun seç.', boardgame: 'Skor tablosu oyunu', loading: 'Skorlar yükleniyor…', rank: 'Sıra', player: 'Oyuncu', score: 'En yüksek skor',
    periodic: 'Skorlar sayfa açıldığında veya Yenile ile alınır. Servise ulaşılamazsa son erişilen kopya gösterilir.', personal: 'Her oyun için kişisel rekorlar', share: "Tabloda yer almak ister misin? DevCade'in Ayarlar menüsünde kullanıcı adını seç ve global skor paylaşımını aç. Paylaşım varsayılan olarak kapalıdır.",
    empty: 'Henüz skor yok. İlk rekoru sen kır!', error: 'Skorlara şu anda ulaşılamıyor. Biraz sonra Yenile ile tekrar deneyebilirsin.', stale: 'Güncel skorlar alınamadı. Erişilebilen son skorlar gösteriliyor.', updated: 'Güncelleme',
    built: 'cagridursun tarafından geliştirildi', creator: 'Küçük oyunlar. Güzel molalar.', creatorcopy: "Ben Çağrı Dursun. DevCade'i, bir yazılımcının günündeki kısa molalar için geliştirdim. Yeni oyunlar ve projeden haberler için beni takip edebilirsin.",
    follow: "X / Twitter'da takip et", footer: 'Yazılımcılar için geliştirildi. MIT lisanslı.', feedback: 'Görüş ve fikirlerini paylaş ↗',
  },
};

const preview = initSnakePreview();
const usage = initUsage();

let language = 'en';
let selected = 'snake';
let snapshot = null;
let state = 'loading';
let busy = false;
const status = document.querySelector('#board-status');
const rows = document.querySelector('#rows');
const table = document.querySelector('#table-wrap');
const updated = document.querySelector('#updated');
const t = (key) => messages[language][key];
const validLanguage = (value) => value === 'en' || value === 'tr';

function renderBoard() {
  document.querySelector('#board-title').textContent = gameNames[selected];
  document.querySelector('#board-caption').textContent = `${gameNames[selected]} — ${t('leaderboard')}`;
  rows.replaceChildren();
  table.hidden = true;
  updated.textContent = '';
  if (state === 'loading') { status.textContent = t('loading'); return; }
  if (state === 'error' || !snapshot) { status.textContent = t('error'); return; }
  const board = snapshot.boards[selected];
  if (board.status === 'unavailable') { status.textContent = t('error'); return; }
  updated.textContent = `${t('updated')}: ${new Intl.DateTimeFormat(language === 'tr' ? 'tr-TR' : 'en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(board.updated_at))}`;
  const stale = isStale(board);
  status.textContent = stale ? t('stale') : board.rows.length ? '' : t('empty');
  if (!board.rows.length) return;
  for (const row of board.rows) {
    const tr = document.createElement('tr');
    if (row.rank === 1) tr.className = 'champion';
    for (const [index, value] of [String(row.rank).padStart(2, '0'), row.username, new Intl.NumberFormat(language).format(row.score)].entries()) {
      const td = document.createElement('td');
      td.textContent = value;
      if (index === 2) td.className = 'score';
      tr.append(td);
    }
    rows.append(tr);
  }
  table.hidden = false;
}

function setLanguage(next) {
  if (!validLanguage(next)) return;
  language = next;
  document.documentElement.lang = language;
  document.title = language === 'tr' ? 'DevCade — Terminalinde küçük bir arcade' : 'DevCade — A little arcade in your terminal';
  for (const element of document.querySelectorAll('[data-i18n]')) {
    const text = t(element.dataset.i18n);
    if (Array.isArray(text)) {
      element.replaceChildren(document.createTextNode(text[0]), document.createElement('br'), document.createTextNode(text[1]));
    } else element.textContent = text;
  }
  for (const element of document.querySelectorAll('[data-i18n-alt]')) element.alt = t(element.dataset.i18nAlt);
  for (const element of document.querySelectorAll('[data-i18n-aria]')) element.setAttribute('aria-label', t(element.dataset.i18nAria));
  for (const button of document.querySelectorAll('[data-language]')) button.setAttribute('aria-pressed', String(button.dataset.language === next));
  try { localStorage.setItem('devcade-page-language', next); } catch { /* Private browsing may disable storage. */ }
  document.querySelector('#copy-status').textContent = '';
  preview.setLabels({ play: t('playAnimation'), pause: t('pauseAnimation'), alt: t('animationAlt') });
  usage.setLabels({ on: t('usageOn'), off: t('usageOff') });
  renderBoard();
}

async function loadScores() {
  if (busy) return;
  busy = true;
  state = 'loading';
  document.querySelector('#refresh').disabled = true;
  renderBoard();
  try {
    snapshot = await loadBoards({ endpoint: 'https://devcade.cinesdigital.com', previous: snapshot,
      snapshotURL: new URL('./leaderboards.json', import.meta.url) });
    state = 'ready';
  } catch { state = 'error'; }
  finally {
    busy = false;
    document.querySelector('#refresh').disabled = false;
    renderBoard();
  }
}

for (const button of document.querySelectorAll('[data-language]')) button.addEventListener('click', () => {
  setLanguage(button.dataset.language);
  const url = new URL(window.location.href);
  url.searchParams.set('lang', language);
  try { history.replaceState(null, '', url); } catch { /* The language switch still works. */ }
});
for (const button of document.querySelectorAll('[data-game]')) button.addEventListener('click', () => {
  selected = button.dataset.game;
  for (const item of document.querySelectorAll('[data-game]')) item.setAttribute('aria-pressed', String(item === button));
  renderBoard();
});
for (const button of document.querySelectorAll('[data-copy]')) button.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(document.getElementById(button.dataset.copy).textContent.trim());
    usage.track('install_copy');
    button.textContent = t('copied');
    document.querySelector('#copy-status').textContent = t('copied');
    setTimeout(() => { button.textContent = t('copy'); }, 1800);
  } catch { document.querySelector('#copy-status').textContent = t('copyFail'); }
});
document.querySelector('#refresh').addEventListener('click', loadScores);
let initial = new URLSearchParams(window.location.search).get('lang');
if (!validLanguage(initial)) {
  try { initial = localStorage.getItem('devcade-page-language'); } catch { /* Default to English. */ }
}
setLanguage(validLanguage(initial) ? initial : 'en');
loadScores();
setInterval(() => { if (state === 'ready') renderBoard(); }, 60000);
