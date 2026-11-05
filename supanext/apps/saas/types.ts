export type Theme = "light" | "dark";

export interface SaasConfig {
	/**
	 * Human-readable product name shown in navigation, metadata, and other
	 * user-facing surfaces throughout the SaaS application.
	 */
	appName: string;
	/**
	 * Absolute URL for the documentation site. When omitted, documentation links
	 * are hidden from the SaaS UI.
	 */
	docsUrl?: string;
	/**
	 * Absolute URL for the public marketing site used by shared navigation links.
	 */
	marketingUrl?: string;
	/**
	 * Theme options exposed in the theme switcher for this app.
	 */
	enabledThemes: readonly Theme[];
	/**
	 * Theme applied when the user has not explicitly chosen one yet.
	 */
	defaultTheme: Theme;
	/**
	 * Controls whether authenticated pages render with the sidebar shell instead
	 * of the simpler top navigation layout.
	 */
	useSidebarLayout: boolean;
	/**
	 * Internal path users are sent to after a successful sign-in flow.
	 */
	redirectAfterSignIn: string;
	/**
	 * Internal path users are sent to after logging out of the application.
	 */
	redirectAfterLogout: string;
}
