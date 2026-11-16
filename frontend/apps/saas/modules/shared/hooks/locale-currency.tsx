import { config as i18nConfig } from "@repo/i18n";
import { useLocale } from "next-intl";

export function useLocaleCurrency() {
	const locale = useLocale();
	const localeCurrency =
		Object.entries(i18nConfig.locales).find(([key]) => key === locale)?.[1]
			.currency ?? i18nConfig.defaultCurrency;

	return localeCurrency;
}
