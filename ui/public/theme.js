/* Native theme preference. Canonical source: nearprod/ui/public/theme.js.
 * Loaded synchronously in <head>, before CSS and app hydration. */
(() => {
  'use strict';
  const key = 'nearprod.theme';
  const root = document.documentElement;
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const valid = (value) => ['system', 'light', 'dark'].includes(value);
  const read = () => {
    try { const value = localStorage.getItem(key); return valid(value) ? value : 'system'; }
    catch { return 'system'; }
  };
  let preference = read();
  function apply() {
    const resolved = preference === 'system' ? (media.matches ? 'dark' : 'light') : preference;
    root.dataset.theme = resolved;
    root.dataset.themePreference = preference;
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.content = resolved === 'dark' ? '#181c22' : '#f4f5f7';
    document.querySelectorAll('[data-theme-picker]').forEach((picker) => { picker.value = preference; });
  }
  apply();
  document.addEventListener('DOMContentLoaded', apply, { once: true });
  document.addEventListener('change', (event) => {
    const picker = event.target;
    if (!(picker instanceof HTMLSelectElement) || !picker.hasAttribute('data-theme-picker') || !valid(picker.value)) return;
    preference = picker.value;
    try { localStorage.setItem(key, preference); } catch { /* The current session still works. */ }
    apply();
  });
  media.addEventListener('change', () => { if (preference === 'system') apply(); });
  window.addEventListener('storage', (event) => {
    if (event.key === key || event.key === null) { preference = read(); apply(); }
  });
})();
