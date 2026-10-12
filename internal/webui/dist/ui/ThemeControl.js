// The shared head script owns preference, persistence and system changes.
// An uncontrolled native select also works before React attaches handlers.
export function ThemeControl() {
    return React.createElement("label", { className: "theme-control" },
        React.createElement("span", null, "Tema"),
        React.createElement("select", { "aria-label": "Tema de apariencia", "data-theme-picker": true, defaultValue: document.documentElement.dataset.themePreference || 'system' },
            React.createElement("option", { value: "system" }, "Sistema"),
            React.createElement("option", { value: "light" }, "Claro"),
            React.createElement("option", { value: "dark" }, "Oscuro")));
}
