export { config, type Locale } from "./config";
export { getMessagesForLocale } from "./lib/get-messages";
export { default as defaultMailTranslations } from "./translations/en/mail.json";
export type {
	MailMessages,
	MarketingMessages,
	SaasMessages,
	SharedMessages,
} from "./types";
