/**
 * Configuration surface for the docs application.
 *
 * The docs app does not currently expose configurable runtime options, but the
 * dedicated interface keeps the config module aligned with the rest of the
 * monorepo and gives future settings a single home.
 */

// biome-ignore lint/complexity/noBannedTypes: remove this once we have config
export type DocsConfig = {};
