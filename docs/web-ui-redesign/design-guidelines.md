# Wire-Pod Web UI Redesign Guidelines

This redesign is anchored on the saved operator-console concept in `docs/web-ui-redesign/operator-console-concept-dark.png`. The goal is not to reskin the old web UI. The current UI exposes useful server and robot controls, but it makes users hunt through giant icon tiles, hidden sections, blocking alerts, and long forms with weak hierarchy. The new UI should feel like a practical control surface for running a local Vector robot server.

## Product Shape

Wire-Pod should open directly into an operator console. A user should be able to answer the basic questions immediately: is the server running, is a robot connected, is speech configured, are logs healthy, and what actions are safe to perform right now?

The main application should use a persistent navigation shell with these areas: Overview, Robot, Setup, Voice, Knowledge, Intents, Logs, Updates, and Appearance. On narrow screens this can collapse into a top bar plus bottom navigation, but it should not become a wall of equally important tiles. Navigation should describe destinations, while the page body should contain the actual work.

The Overview page should combine robot status, server health, recent activity, and safe quick actions. Setup pages should be guided and explanatory. Advanced controls should remain available, but they should be separated from common actions and visually marked when they can move the robot, change credentials, restart services, delete data, or affect safety.

Vector's live state should be a first-class product feature, not a footnote. The console should explain what the robot/server pipeline appears to be doing in plain language: connected or unavailable, docked or off charger, waiting for wake word, processing speech, matching an intent, consulting the LLM, executing an action, or idle. When the frontend only has indirect evidence, the UI should say so honestly, for example "No recent activity" or "Waiting state unavailable from current backend." Derived state can come from the robot SDK, battery endpoint, server configuration, and recent logs, but the UI must distinguish confirmed state from inferred state.

## Visual System

The design should be dense enough for an admin tool, but not cramped. Use a restrained card-and-panel system with 8px radii, clear section headers, and predictable spacing. Avoid nested cards. A page may contain panels, lists, forms, tables, and status strips, but page sections themselves should not look like floating marketing cards.

The dark theme should use a near-black application background, slightly elevated panels, soft borders, muted text, and a controlled green accent. The green should signal health and primary actions, not flood the interface. Amber should indicate setup warnings or attention-needed states, and red should be reserved for destructive or urgent robot controls.

The light theme should not be a color inversion. It should use an off-white or very pale gray background, white panels, graphite text, soft gray borders, the same green accent, and sparing amber/red status colors. The relative hierarchy should match dark mode: panels, sidebars, inputs, and active navigation must still be obvious.

Use the existing local robot face assets where they help communicate state. Do not depend on heavy illustration or generated artwork for core operation. If a larger robot visual is needed, it should be lightweight, replaceable, and not block useful status content.

## Interaction Rules

Avoid blocking `alert()` for normal success and error feedback. Use inline status messages, banners, or toast-style messages that preserve context. Browser confirms are acceptable as a temporary guard for destructive actions, but the redesigned surface should prefer explicit danger buttons and clear confirmation copy for destructive workflows.

Forms should be grouped by task. Labels, helper text, validation, and save buttons should stay near the controls they affect. Long explanatory copy should be shortened into practical helper text, and advanced fields should be revealed only when the selected provider or mode requires them.

Safe action controls should be visually separate from risky controls. Waking the robot, docking it, saying text, or taking a photo can be everyday actions. Restarting the server, deleting chats, deleting faces/photos, sending arbitrary audio, behavior-control override, movement controls, and emergency stop need stronger affordances and context.

Theme switching should be first-class. The selected theme should persist in `localStorage`, respect the user's system preference on first load, and update without a page reload. Components must derive colors from CSS tokens instead of one-off hard-coded colors.

## Implementation Principles

Keep the implementation compatible with the existing static webroot served by the Go binary. A full frontend build system is not required for the first redesign pass. Prefer a small reusable design system in plain HTML, CSS, and JavaScript: layout shell, buttons, panels, status pills, form rows, switches, meters, data lists, activity rows, empty states, and responsive navigation.

Preserve existing backend endpoints and request payloads unless there is a deliberate server change. The redesign may reorganize pages and controls, but it must not silently drop existing flows: initial setup, server settings, speech-to-text settings, knowledge/weather providers, custom intents, logs, version/update checks, robot selection, robot settings, robot control, photos, faces, locale/time/unit settings, Alexa, location/timezone, and robot action endpoints.

Checkpoint deployments should copy the updated webroot into the running local Docker container named `wire-pod` and verify the live app through the real container server. The container serves the web UI on `http://localhost:8080/` in this checkout. Rebuilding the image is only necessary when Go/server code changes.

## Browser QA Bar

Every stable implementation checkpoint should be checked in a real browser at desktop and mobile widths. Verification should cover layout quality, theme switching, main navigation, status rendering with missing or failing API data, setup forms, log display, and robot-control affordances. A page that only looks good in a static screenshot is not complete.

The final audit should prove the requested state from the active goal: concept-aligned operator console, light and dark themes, clear navigation, robot/server status, guided setup, logs, safe action controls, reusable design system, local Docker deployment, and real browser validation across desktop and mobile.
