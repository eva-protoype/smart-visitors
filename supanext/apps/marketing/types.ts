export type Theme = "light" | "dark";

export interface MarketingConfig {
	/**
	 * Human-readable product name used in site chrome, SEO metadata, and landing
	 * page copy.
	 */
	appName: string;
	/**
	 * Absolute URL for the docs site. When omitted, documentation calls to action
	 * are not rendered.
	 */
	docsUrl?: string;
	/**
	 * Absolute URL for the SaaS application that marketing pages use for sign-in,
	 * dashboard, and conversion links.
	 */
	saasUrl?: string;
	/**
	 * Theme options available on the marketing site.
	 */
	enabledThemes: readonly Theme[];
	/**
	 * Theme used for first-time visitors before a preference is stored.
	 */
	defaultTheme: Theme;
}
