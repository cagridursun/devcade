// Entirely opt-in. No request or identifier is created before consent.
export function initUsage() {
  const button = document.querySelector('#usage-toggle');
  let enabled = false;
  let id = '';
  let visited = false;
  let labels = {on:'Site statistics: on',off:'Site statistics: off'};
  const pending = new Set();
  try { enabled = localStorage.getItem('devcade-site-usage') === 'on'; } catch { /* Default off. */ }
  const randomID = () => crypto.randomUUID().replaceAll('-','');
  function render() { if (!button) return; button.textContent = enabled ? labels.on : labels.off; button.setAttribute('aria-pressed',String(enabled)); }
  function track(kind) {
    if (!enabled || !['site_visit','install_copy'].includes(kind)) return;
    try {
      if (!id) {
        id = sessionStorage.getItem('devcade-site-usage-id') || randomID();
        sessionStorage.setItem('devcade-site-usage-id',id);
      }
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(),3000);
      pending.add(controller);
      fetch('https://devcade.cinesdigital.com/v1/metrics/site', {
        method:'POST', headers:{'Content-Type':'application/json'}, credentials:'omit', redirect:'error',referrerPolicy:'no-referrer',signal:controller.signal,
        body:JSON.stringify({id:randomID(),installation:id,kind,platform:'browser',version:'website'})
      }).catch(() => {}).finally(() => {clearTimeout(timer);pending.delete(controller);});
    } catch { /* Storage/crypto/network failures never affect the website. */ }
  }
  function visit() { if (enabled && !visited) { visited = true; track('site_visit'); } }
  button?.addEventListener('click', () => {
    enabled = !enabled;
    try { localStorage.setItem('devcade-site-usage',enabled?'on':'off'); } catch { /* Consent applies to this page only. */ }
    if (!enabled) {
      for (const controller of pending) controller.abort();
      pending.clear(); id = '';
      try { sessionStorage.removeItem('devcade-site-usage-id'); } catch { /* No stored ID. */ }
    }
    render(); visit();
  });
  render();visit();
  return { track, setLabels(next) { labels = next; render(); } };
}
