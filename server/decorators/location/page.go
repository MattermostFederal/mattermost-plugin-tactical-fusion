package location

// pageStyles declares the Mattermost theme variables the standalone pages need.
//
// The page renders the same React components the sidebar renders, and those
// style themselves from Mattermost's own custom properties. A page has no
// Mattermost around it, so it declares them itself, in both themes, the way the
// shared shell declares its own. Everything else a coordinate looks like is in
// the components.
//
// The map itself no longer follows this. It is drawn dark in both themes, by
// ALWAYS_DARK in maplibre.ts, so a light page carries a dark map. What these
// properties still decide is everything AROUND it: the table, the links and the
// page's own ground. There is still no second copy of a measured palette here.
const pageStyles = `
:root {
  --center-channel-color: #3f4350; --center-channel-color-rgb: 63,67,80;
  --center-channel-bg: #ffffff; --link-color: #1c58d9;
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --center-channel-color: #dddfe4; --center-channel-color-rgb: 221,223,228;
    --center-channel-bg: #1b1d22; --link-color: #7ba7ff;
  }
}
:root[data-theme="dark"] {
  --center-channel-color: #dddfe4; --center-channel-color-rgb: 221,223,228;
  --center-channel-bg: #1b1d22; --link-color: #7ba7ff;
}
`
