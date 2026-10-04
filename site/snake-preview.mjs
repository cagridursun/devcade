export function cleanReplay(value) {
  if (!value || value.version !== 1 || value.cols !== 80 || value.rows !== 24 ||
    !Array.isArray(value.frames) || value.frames.length < 2 || value.frames.length > 500) throw new Error('Invalid replay');
  let duration = 0;
  for (const frame of value.frames) {
    if (!Number.isInteger(frame.duration) || frame.duration < 50 || frame.duration > 250 ||
      !Array.isArray(frame.text) || frame.text.length !== 24 || !Array.isArray(frame.colors) || frame.colors.length !== 24) throw new Error('Invalid frame');
    for (let row = 0; row < 24; row++) {
      if (typeof frame.text[row] !== 'string' || !/^[ -~]{80}$/.test(frame.text[row]) ||
        typeof frame.colors[row] !== 'string' || !/^[0-3]{80}$/.test(frame.colors[row])) throw new Error('Invalid terminal row');
    }
    duration += frame.duration;
  }
  if (duration > 100000) throw new Error('Replay too long');
  return { ...value, duration };
}

export function frameAt(replay, elapsed) {
  let time = Math.max(0, elapsed) % replay.duration;
  for (let index = 0; index < replay.frames.length; index++) {
    if (time < replay.frames[index].duration) return index;
    time -= replay.frames[index].duration;
  }
  return 0;
}

export function initSnakePreview() {
  const element = document.querySelector('#snake-preview');
  const poster = document.querySelector('#snake-poster');
  const button = document.querySelector('#animation-toggle');
  const ctx = element?.getContext('2d');
  let labels = { play: 'Play animation', pause: 'Pause animation', alt: 'Animated replay of Snake gameplay' };
  const controller = { setLabels(next) { labels = next; updateLabel(); } };
  function updateLabel() {
    if (!button || !element) return;
    button.textContent = playing ? labels.pause : labels.play;
    element.setAttribute('aria-label', labels.alt);
  }
  // Keep the original screenshot if JavaScript, canvas or the recording fails.
  let playing = false;
  if (!ctx || !poster || !button) return controller;
  const motion = matchMedia('(prefers-reduced-motion: reduce)');
  let replay = null;
  let visible = false;
  let elapsed = 0;
  let lastTime = null;
  let lastFrame = -1;
  let request = null;
  const colors = ['#d8dee9', '#66b5ca', '#a6d86e', '#e5c07b'];

  function draw(index) {
    if (index === lastFrame) return;
    lastFrame = index;
    element.dataset.frame = String(index);
    ctx.fillStyle = '#101419';
    ctx.fillRect(0, 0, 1000, 664);
    ctx.textBaseline = 'top';
    ctx.font = '17px ui-monospace, SFMono-Regular, Consolas, monospace';
    const frame = replay.frames[index];
    for (let y = 0; y < 24; y++) for (let x = 0; x < 80; x++) {
      if (frame.text[y][x] === ' ') continue;
      ctx.fillStyle = colors[Number(frame.colors[y][x])];
      ctx.fillText(frame.text[y][x], 20 + x * 12, 20 + y * 26);
    }
  }

  function tick(time) {
    request = null;
    if (!playing || !visible || document.hidden || !replay) { lastTime = null; return; }
    if (lastTime !== null) elapsed = (elapsed + Math.min(100, Math.max(0, time - lastTime))) % replay.duration;
    lastTime = time;
    draw(frameAt(replay, elapsed));
    request = requestAnimationFrame(tick);
  }
  function sync() {
    element.dataset.playing = String(playing && visible && !document.hidden);
    if (request !== null) cancelAnimationFrame(request);
    request = null;
    lastTime = null;
    if (replay && playing && visible && !document.hidden) request = requestAnimationFrame(tick);
    updateLabel();
  }
  button.addEventListener('click', () => { playing = !playing; sync(); });
  document.addEventListener('visibilitychange', sync);
  motion.addEventListener('change', () => { playing = !motion.matches; sync(); });
  const observer = new IntersectionObserver(([entry]) => { visible = entry.isIntersecting; sync(); }, { threshold: 0.05 });
  observer.observe(element.parentElement);

  (async () => {
    try {
      const response = await fetch(new URL('./assets/snake-demo.json', import.meta.url), { credentials: 'omit', signal: AbortSignal.timeout(10000) });
      if (!response.ok) throw new Error('Replay unavailable');
      const text = await response.text();
      if (text.length > 2_000_000) throw new Error('Replay too large');
      replay = cleanReplay(JSON.parse(text));
      draw(0);
      element.hidden = false;
      poster.hidden = true;
      button.hidden = false;
      playing = !motion.matches;
      sync();
    } catch {
      observer.disconnect();
      // The visible static fallback already supplies the game's description.
    }
  })();
  return controller;
}
