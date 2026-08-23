/**
 * What the stubbed package route answers, shared by the harness and its tests.
 *
 * Separate from the harness because Playwright CT rewrites a component import
 * and will not mount a component whose module also exports something else.
 */

/** Two areas, one of them in the drop-in directory. */
export const INSTALLED = ['indopacom-guam', 'indopacom-hawaii'];

/** The subset Remove is offered for. The other one ships inside the plugin. */
export const REMOVABLE = ['indopacom-guam'];

/** A drop-in build of an area the plugin also ships, which is what shadows it. */
export const SHADOWING = ['indopacom-hawaii'];

/**
 * What the list becomes once an upload has landed: the new area is in the
 * drop-in directory and so removable, the bundled one beside it is not.
 */
export const AFTER_UPLOAD = {packages: ['eucom-baltics', 'indopacom-hawaii'], removable: ['eucom-baltics']};

/**
 * What it becomes once a drop-in has been removed. Where that drop-in shadowed
 * a bundled area the name stays listed, because the bundled one underneath
 * resurfaces, and it is no longer removable.
 */
export const AFTER_REMOVAL = {packages: ['indopacom-hawaii'], removable: []};

/** A refusal the server words itself, which the control must show rather than replace. */
export const REFUSAL = 'that area ships inside the plugin and is replaced by a release, not from here';
