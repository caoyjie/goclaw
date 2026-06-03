# WebUI Brand & Visual Renovation Specification (CEO IP Assistant)

## 1. Objective
To unify the Web Dashboard (`ui/web`) under the `CEO IP Assistant` brand identity, while ensuring technical protocol and runtime compatibility remain unaffected.

## 2. Scope of Application
Applies only to: `ui/web`

Excludes:
- Backend protocol fields (e.g., `X-GoClaw-*`)
- Legacy localStorage keys (e.g., `goclaw:*`)
- Environment variable names (e.g., `GOCLAW_*`)
- Desktop frontend (`ui/desktop`)

## 3. Finalized Requirements

### 3.1 Brand Copy
- The frontend display layer brand is uniformly named: `CEO IP Assistant`
- Remove brand logo display from pages (retain text-based brand only)

### 3.2 Language Options
- Keep only the following languages in the dropdown:
  - `English`
  - `中文`
- Remove Vietnamese (`vi`) as an optional language

### 3.3 Color Scheme

#### Light Mode
- Page background: pure white
- Cards, input fields, option areas: light blue tones
- Primary accent color: blue tones

#### Dark Mode
- Keep the current dark mode scheme without additional changes (follow the existing dark blue dark mode logic)

### 3.4 Sidebar (Key Focus)
- Background: dark blue
- Text: white (including titles, group headings, connection status)
- Hover state: medium-light blue background + white text

## 4. Implementation Strategy
1. Prioritize unifying colors through global design tokens (`index.css`).
2. Add white-text classes to sidebar components to prevent being overridden by `muted/foreground` variables.
3. Replace only user-visible strings for copy changes, leaving technical identifiers unchanged.

## 5. Acceptance Criteria

### 5.1 Visual
- In light mode, the page background is pure white.
- The sidebar has a dark blue background, and all text is white.
- Hovering over navigation items shows a clear blue highlight with readable text.

### 5.2 Language
- The top‑bar language dropdown contains only `English` and `中文`.
- If an existing user has the language stored locally as `vi`, the page should fall back to `en` (or follow browser rules to `en/zh`).

### 5.3 Functional Integrity
- Login, navigation, sessions, and system settings pages remain usable.
- No new frontend errors are introduced.
- API calls and authentication headers are unaffected.

## 6. Regression Checklist
- Login page: brand copy is correct, no logo.
- Sidebar:
  - The top `CEO IP Assistant` is readable in white.
  - Group headings (e.g., "Core") are readable in white.
  - The bottom connection status (e.g., "Connected") is readable in white.
- Top‑bar language switcher: only Chinese and English.
- Light/Dark theme toggle: light mode has a white background; dark mode preserves the existing style.