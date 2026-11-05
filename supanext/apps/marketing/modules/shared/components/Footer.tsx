import { config } from "@config";
import { LocaleLink } from "@i18n/routing";
import { Logo } from "@repo/ui";
import { useTranslations } from "next-intl";

export function Footer() {
	const t = useTranslations();

	return (
		<footer className="border-t py-8 text-foreground/60 text-sm">
			<div className="container grid grid-cols-1 gap-6 lg:grid-cols-3">
				<div>
					<Logo className="opacity-70 grayscale" />
					<p className="mt-3 text-sm opacity-70">
						© {new Date().getFullYear()} {config.appName}.{" "}
						<a href="https://supastarter.dev">
							{t("common.footer.builtWith")}
						</a>
						.
					</p>
				</div>

				<div className="flex flex-col gap-2">
					<LocaleLink href="/blog" className="block">
						{t("common.footer.blog")}
					</LocaleLink>

					<a href="#features" className="block">
						{t("common.footer.features")}
					</a>

					<a href="/#pricing" className="block">
						{t("common.footer.pricing")}
					</a>
				</div>

				<div className="flex flex-col gap-2">
					<LocaleLink href="/legal/privacy-policy" className="block">
						{t("common.footer.privacyPolicy")}
					</LocaleLink>

					<LocaleLink href="/legal/terms" className="block">
						{t("common.footer.termsAndConditions")}
					</LocaleLink>
				</div>
			</div>
		</footer>
	);
}
