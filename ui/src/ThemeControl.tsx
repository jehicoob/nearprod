// The shared head script owns preference, persistence and system changes.
// An uncontrolled native select also works before React attaches handlers.
export function ThemeControl() {
  return <label className="theme-control">
    <span>Tema</span>
    <select aria-label="Tema de apariencia" data-theme-picker defaultValue={document.documentElement.dataset.themePreference || 'system'}>
      <option value="system">Sistema</option>
      <option value="light">Claro</option>
      <option value="dark">Oscuro</option>
    </select>
  </label>;
}
