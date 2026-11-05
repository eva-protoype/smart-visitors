"use client";

import { updateLocale } from "@i18n/lib/update-locale";
import type { Locale } from "@repo/i18n";
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
import { useRouter } from "next/navigation";
import { useLocale } from "next-intl";
import { useState } from "react";

export function LocaleSwitch() {
	const router = useRouter();
	const currentLocale = useLocale();
	const [value, setValue] = useState<string>(currentLocale);

	if (Object.keys(i18nConfig.locales).length <= 1) {
		return null;
	}

	return (
		<DropdownMenu modal={false}>
			<DropdownMenuTrigger asChild>
				<Button variant="ghost" size="icon" aria-label="Language">
					<LanguagesIcon className="size-4" />
				</Button>
			</DropdownMenuTrigger>

			<DropdownMenuContent>
				<DropdownMenuRadioGroup
					value={value}
					onValueChange={(value) => {
						setValue(value);
						updateLocale(value as Locale);
						router.refresh();
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
