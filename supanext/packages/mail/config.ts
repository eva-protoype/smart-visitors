import { config as i18nConfig } from "@repo/i18n";
import type { MailConfig } from "./types";

export const config = {
	mailFrom: process.env.MAIL_FROM as string,
	locales: Object.keys(
		i18nConfig.locales,
	) as (keyof typeof i18nConfig.locales)[],
	defaultLocale: i18nConfig.defaultLocale,
} satisfies MailConfig;

export type Locale = keyof typeof i18nConfig.locales;
