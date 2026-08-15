package location

// pageStyles declares the Mattermost theme variables the standalone pages need.
//
// The page renders the same React components the sidebar renders, and those
// style themselves from Mattermost's own custom properties. A page has no
// Mattermost around it, so it declares them itself, in both themes, the way the
// shared shell declares its own. Everything else a coordinate looks like is in
// the components.
//
// This is also what makes the map follow the page's theme: mapColors reads
// --center-channel-bg to decide which palette to draw, so there is no second
// copy of a measured palette here to keep in step.
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
