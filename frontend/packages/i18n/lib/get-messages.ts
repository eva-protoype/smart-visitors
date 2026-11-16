import { toMerged } from "es-toolkit";
import { config, type Locale } from "../config";

export type TranslationScope = "marketing" | "saas" | "mail";

async function importLocaleMessages<T>(
	locale: Locale,
	scope: TranslationScope | "shared",
): Promise<T> {
	return (await import(`../translations/${locale}/${scope}.json`))
		.default as T;
}

export async function getMessagesForLocale<T = Record<string, unknown>>(
	locale: Locale,
	scope: TranslationScope,
): Promise<T> {
	const localeMessages = await importLocaleMessages<T>(locale, scope);

	const sharedMessages = await importLocaleMessages<Record<string, unknown>>(
		locale,
		"shared",
	);

	let messages = toMerged(
		localeMessages as Record<string, unknown>,
		sharedMessages,
	) as T;

	if (locale !== config.defaultLocale) {
		const defaultLocaleMessages = await importLocaleMessages<T>(
			config.defaultLocale,
			scope,
		);
		const defaultSharedMessages = await importLocaleMessages<
			Record<string, unknown>
		>(config.defaultLocale, "shared");
		const defaultMessages = toMerged(
			defaultLocaleMessages as Record<string, unknown>,
			defaultSharedMessages,
		);
		messages = toMerged(
			defaultMessages,
			messages as Record<string, unknown>,
		) as T;
	}

	return messages;
}
