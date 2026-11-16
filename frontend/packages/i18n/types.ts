import type mailMessages from "./translations/en/mail.json";
import type marketingMessages from "./translations/en/marketing.json";
import type saasMessages from "./translations/en/saas.json";
import type sharedMessages from "./translations/en/shared.json";

export interface LocaleDefinition {
	/**
	 * Human-readable locale name displayed in language selectors and settings UIs.
	 */
	label: string;
	/**
	 * ISO currency code used for locale-specific pricing defaults.
	 */
	currency: string;
}

export interface I18nConfig {
	/**
	 * Supported locales keyed by locale code. Each entry controls how the locale
	 * appears in selectors and which default currency is paired with it.
	 */
	locales: Record<string, LocaleDefinition>;
	/**
	 * Locale used when no locale segment, cookie, or explicit preference is
	 * available.
	 */
	defaultLocale: string;
	/**
	 * Fallback currency used when a locale-specific currency cannot be resolved.
	 */
	defaultCurrency: string;
	/**
	 * Cookie name that stores the user's preferred locale between requests.
	 */
	localeCookieName: string;
}

export type SharedMessages = typeof sharedMessages;
export type MarketingMessages = typeof marketingMessages & SharedMessages;
export type SaasMessages = typeof saasMessages & SharedMessages;
export type MailMessages = typeof mailMessages;
