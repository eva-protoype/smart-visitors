import type { MailMessages } from "@repo/i18n";

declare global {
	interface IntlMessages extends MailMessages {}
}
