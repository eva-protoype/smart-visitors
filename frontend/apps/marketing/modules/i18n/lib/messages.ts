import type { MarketingMessages } from "@repo/i18n";
import { getMessagesForLocale as getMessages } from "@repo/i18n";

export const getMessagesForLocale = async (
	locale: string,
): Promise<MarketingMessages> => {
	return getMessages(
		locale as Parameters<typeof getMessages>[0],
		"marketing",
	);
};
