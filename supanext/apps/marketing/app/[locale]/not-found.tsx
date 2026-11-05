import { LocaleLink } from "@i18n/routing";
import { Button } from "@repo/ui";
import { ArrowLeftIcon } from "lucide-react";
import { getTranslations } from "next-intl/server";

export default async function NotFoundPage() {
	const t = await getTranslations("notFound");

	return (
		<div className="flex h-full flex-col items-center justify-center">
			<h1 className="font-bold text-5xl">{t("code")}</h1>
			<p className="mt-2 text-2xl">{t("title")}</p>

			<Button asChild className="mt-4">
				<LocaleLink href="/">
					<ArrowLeftIcon className="mr-2 size-4" />{" "}
					{t("goToHomepage")}
				</LocaleLink>
			</Button>
		</div>
	);
}
