# NearProd product design

Status: approved direction, 2026-10-10. The user approved adopting the landing's
identity in the platform and explicitly requested this contract inside `ui/`.
This file is the canonical NearProd brand and platform design contract; its
location intentionally supersedes the usual repository-root convention.

## Purpose and authority

NearProd is a local controller for Compose projects. The interface must make
configuration, execution, health and access distinguishable. A completed command
does not imply a healthy application. Preserve the actual backend states and
existing authentication, trust review and action confirmations.

The approved landing supplied the brand direction. This contract now owns its
shared foundation. The separate `nearprod-landing` repository's `DESIGN.md` owns
only marketing composition, copy and expressive scroll behavior; it references
this contract for brand decisions. Skills provide reusable methods, not NearProd
identity or a second design authority.

## Approved palette revision — 2026-10-10

The user approved the recommended graphite and muted steel direction for both
the landing and platform with “implementala en ambos”. It supersedes the previous
cream/forest/lime foundation. Preserve layout, typography, motion and operational
semantics. Neutral surfaces dominate; steel identifies primary actions and focus.
Keep separate accent roles for solid buttons, pale highlights and dark surfaces.
Green, amber and red remain status colors, not general brand decoration.

## Shared foundation

The executable source is [public/brand.css](public/brand.css). It defines neutral paper,
white surfaces, graphite ink, muted text, steel accents and line colors, system font families,
corner radius and feedback timing. Do not reproduce an independent palette in a
component. [public/icon.svg](public/icon.svg) is the shared layered-panel mark.
Use the lowercase `nearprod.` wordmark for visual brand lockups; product names in
prose and accessible labels remain `NearProd`.

The landing vendors the brand, icon and native theme controller through its `scripts/sync-brand.mjs` command,
with an explicit path to this repository. Its aliases refer to the `--brand-*`
tokens. Synchronization copies the canonical assets exactly and can check drift;
neither app depends on a sibling checkout at runtime or on a remote font/service.
Review both consumers when changing the shared foundation.

## Operational surface

[public/theme.css](public/theme.css) maps the shared brand to semantic roles:

- Light theme: graphite sidebar, topbar and code/log surfaces; light gray workspace and white cards. The paired dark theme is defined below.
- Graphite text, readable neutral metadata and muted steel primary actions with white labels.
- Pale steel belongs to subtle selection and illustration fills; a lighter steel accent
  is used for links, selection indicators and focus against graphite surfaces.
- Green success, amber caution, red failure/destructive actions, teal development
  and violet verification states. Labels and icons carry meaning as well as color.
- System sans for controls and reading, monospace for commands, paths and technical
  metadata. Serif is available to marketing; do not impose it on dense controls.
- Compact operational spacing, quiet borders and restrained elevation for dialogs.
  Preserve the existing information hierarchy instead of reproducing hero sizes.

Avoid new one-off color literals in [public/styles.css](public/styles.css); extend
semantic roles in `theme.css` when there is a real recurring need. The brand accent
does not replace health/error semantics. Disabled controls must remain recognizable.

## Shell and component reuse

The shell remains in [src/App.tsx](src/App.tsx). Keep the seven destinations and their
existing behavior: Aplicaciones, Accesos locales, Infraestructura, Herramientas, Acceso MCP,
Runtime y recursos, and Actividad. Active navigation has a visible state and
`aria-current`; desktop icon-only navigation retains accessible names and titles.

[src/components.tsx](src/components.tsx) owns `Brand`, `Icon`, `Badge`, `Modal`,
`Alert`, `Busy`, `LiveStatus`, `Skeleton`, and `Field`. Reuse these and the existing
button classes rather than introducing a new component library. The cube remains
an operational icon; it is no longer the brand mark. Form identifiers, status
labels, error content and API interactions retain their established contracts.

The Aplicaciones pilot establishes cards, group headers, action hierarchy,
summary metrics, search and configuration review. Shared roles also apply to
infrastructure, tools, routes, runtime, activity, login and their dialogs.

## Responsive and accessibility behavior

- Desktop: fixed navigation, bounded main reading width, compact task controls.
- Tablet: an icon rail with the same accessible destinations.
- At 600px and below: brand/session controls and an internally scrollable labeled
  navigation strip. Keyboard focus scrolls only that strip until the whole
  destination is visible. Never remove access to a destination at a breakpoint.
- Cards, actions, forms and metadata wrap; long paths must not widen the page.
  Tables and logs may scroll inside their own bounded regions.
- Preserve native dialog focus containment, Escape, close controls and focus return.
  Close the native dialog before React removes it, restoring its connected opener.
  Keep visible keyboard focus on light and graphite surfaces.
- Preserve loading, disconnected, empty, running, failed and unverified states;
  refreshes must not replace existing content with a collapsing placeholder.

## Motion

Use native CSS and browser APIs, without animation dependencies. Feedback uses
the shared 180ms token. Dialogs, log panels and operation details may use a short
opacity entrance; their controls never move while being targeted. Closing and
state changes remain immediate and do not depend on animation events.

Honor `prefers-reduced-motion`, including a change during the session. The existing
loading indicators may animate only while representing real pending work. There
are no ambient loops, pinned scroll scenes, fake progress or synthetic execution
states in this operational surface. The landing keeps its separately approved
expressive scene behavior.

## Build and acceptance

Edit source under `ui/`. Run `npm run typecheck` and `npm run build:ui`; the latter
regenerates the committed assets in `internal/webui/dist` for Go embedding.
Never hand-edit generated distribution files. Preserve vendored React and licenses;
Node remains development tooling, not a requirement of the installed controller.

Acceptance requires a rendered desktop and narrow viewport, login/error/loading,
offline/empty and populated Applications, search, configuration review, navigation,
logs, keyboard focus, dialog close/return, reduced motion and no page overflow.
Use `scripts/e2e.py` for the established browser regression: real React and Go HTTP,
with Docker/Traefik test doubles. Its pass does not establish real Docker behavior.
Check other views affected by shared styles, semantic contrast and brand asset drift.

Do not replace historical marketing screenshots with fixture evidence presented as
a real release. Updated captures need an identified build and truthful data labels.

## Installation compatibility

Before installing a locally built binary, compare its `--identity` (release and
schema) with the installed executable and run `config migrate --dry-run` against
the intended home. Stop on a newer unsupported catalog; never rewrite its schema
number or replace it with an empty catalog. Build the UI on the current compatible
backend branch, preserving MCP, lifecycle, and application-control behavior.
A clean temporary home is insufficient evidence for an existing installation.

## Approved light/dark themes and typography — 2026-10-12

The user approved the paired graphite/steel palette and the typography correction
with “procede”. `public/brand.css` now owns shared semantic light/dark roles while
the original `--brand-*` identity values remain stable. Both products consume the
same foundation and native `public/theme.js` controller. Landing synchronization
vendors that controller too; there is no runtime dependency on a sibling checkout.

- Light: canvas #F4F5F7, cards #FFFFFF, text #242930, muted #5B6573,
  primary #526C8C with white text.
- Dark: canvas #181C22, cards #222831, text #E7EBF0, muted #ABB5C3,
  primary #ADC0D8 with #181C22 text. Inverse/code surfaces remain distinct.
- Status roles retain green/amber/red/teal/violet meaning with theme-specific
  backgrounds and foregrounds. Decorative lines and control boundaries differ.
- Native “Sistema / Claro / Oscuro” controls remain available before and after
  login. System is the default; explicit choices persist per browser origin.
  The shared head script resolves the preference before styles render, handles
  system changes and other tabs, and tolerates unavailable storage. CSS follows
  the system when JavaScript is unavailable. No external theme library is used.
- Operational type uses rem-based roles: body/lists/controls 14px, code/data 13px,
  captions 12px, section headings 16px, page headings 28–32px at default browser
  size. The browser's default root size is preserved for user preferences.
  Adjacent MCP panels share heading and reading roles. No essential mobile text
  is reduced to fit; controls wrap. Arial and the existing mono family remain.
- Marketing retains its own display scale, serif accents, motion and original
  screenshot pixels. The theme does not recolor historical screenshots.

Acceptance covers both themes on desktop/mobile, keyboard selection, reload and
cross-tab persistence, system changes, blocked storage, pre-paint resolution,
MCP computed typography, forms/dialogs, focus, overflow, and existing regressions.


Local validation on 2026-10-12: TypeScript compilation and embedded UI build
passed; the native Go/Chromium browser suite passed 39 checks using a temporary
catalog and simulated Docker/Traefik. Theme coverage includes pre-login and
post-login controls, saved preference before first paint, system and cross-tab
changes, both MCP themes at 320/430/750/1440px, and a larger root font setting.
The shared palette and native controller match the landing copies byte for byte.
No new release or installation is implied by this local acceptance.
