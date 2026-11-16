import { Link, Text } from "@react-email/components";
import React from "react";
import { createTranslator } from "use-intl/core";
import PrimaryButton from "../components/PrimaryButton";
import Wrapper from "../components/Wrapper";
import { defaultLocale, defaultTranslations } from "../lib/translations";
import type { BaseMailProps } from "../types";

export function NewUser({
	url,
	otp,
	locale,
	translations,
}: {
	url: string;
	name: string;
	otp: string;
} & BaseMailProps) {
	const t = createTranslator({
		locale,
		messages: { ...translations.newUser, common: translations.common },
	});

	return (
		<Wrapper>
			<Text>{t("body")}</Text>

			<Text>
				{t("common.otp")}
				<br />
				<strong className="font-bold text-2xl">{otp}</strong>
			</Text>

			<Text>{t("common.useLink")}</Text>

			<PrimaryButton href={url}>{t("confirmEmail")} &rarr;</PrimaryButton>

			<Text className="text-muted-foreground text-sm">
				{t("common.openLinkInBrowser")}
				<Link href={url}>{url}</Link>
			</Text>
		</Wrapper>
	);
}

NewUser.PreviewProps = {
	locale: defaultLocale,
	translations: defaultTranslations,
	url: "#",
	name: "John Doe",
	otp: "123456",
};

export default NewUser;
