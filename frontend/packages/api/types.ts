/**
 * Configuration surface for the shared API package.
 *
 * The package does not currently expose configurable options, but the
 * dedicated interface keeps the config module consistent with the rest of the
 * repository and provides a typed extension point for future settings.
 */

// biome-ignore lint/suspicious/noEmptyInterface: remove this once we have config
export interface ApiConfig {}
