import { config as i18nConfig } from "@repo/i18n";
import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";
import { getMessagesForLocale } from "./lib/messages";

export default getRequestConfig(async ({ requestLocale }) => {
	let locale = await requestLocale;

	if (!locale) {
		const cookieStore = await cookies();
		const localeCookie = cookieStore.get(i18nConfig.localeCookieName);
		locale = localeCookie?.value ?? i18nConfig.defaultLocale;
	}

	return {
		locale,
		messages: await getMessagesForLocale(locale),
	};
});
