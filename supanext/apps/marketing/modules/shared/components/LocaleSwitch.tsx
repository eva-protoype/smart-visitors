"use client";

import { useLocalePathname, useLocaleRouter } from "@i18n/routing";
import { config as i18nConfig } from "@repo/i18n";
import { Button } from "@repo/ui/components/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuTrigger,
} from "@repo/ui/components/dropdown-menu";
import { LanguagesIcon } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useState } from "react";

export function LocaleSwitch() {
	const t = useTranslations();
	const localeRouter = useLocaleRouter();
	const localePathname = useLocalePathname();
	const searchParams = useSearchParams();
	const currentLocale = useLocale();
	const [value, setValue] = useState<string>(currentLocale);

	if (Object.keys(i18nConfig.locales).length <= 1) {
		return null;
	}

	return (
		<DropdownMenu modal={false}>
			<DropdownMenuTrigger asChild>
				<Button
					variant="ghost"
					size="icon"
					aria-label={t("common.aria.language")}
				>
					<LanguagesIcon className="size-4" />
				</Button>
			</DropdownMenuTrigger>

			<DropdownMenuContent>
				<DropdownMenuRadioGroup
					value={value}
					onValueChange={(value) => {
						setValue(value);

						localeRouter.replace(
							`${localePathname}?${searchParams.toString()}`,
							{
								locale: value,
							},
						);
					}}
				>
					{Object.entries(i18nConfig.locales).map(
						([locale, { label }]) => {
							return (
								<DropdownMenuRadioItem
									key={locale}
									value={locale}
								>
									{label}
								</DropdownMenuRadioItem>
							);
						},
					)}
				</DropdownMenuRadioGroup>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
