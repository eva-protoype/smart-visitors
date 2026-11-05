import type { MailMessages } from "@repo/i18n";
import { getMessagesForLocale as getMessages } from "@repo/i18n";

export type Messages = MailMessages;

export const getMessagesForLocale = async (
	locale: Parameters<typeof getMessages>[0],
): Promise<Messages> => {
	return getMessages(locale, "mail");
};
