import { Link, Text } from "@react-email/components";
import React from "react";
import { createTranslator } from "use-intl/core";
import PrimaryButton from "../components/PrimaryButton";
import Wrapper from "../components/Wrapper";
import { defaultLocale, defaultTranslations } from "../lib/translations";
import type { BaseMailProps } from "../types";

export function ForgotPassword({
	url,
	locale,
	translations,
}: {
	url: string;
	name: string;
} & BaseMailProps) {
	const t = createTranslator({
		locale,
		messages: {
			...translations.forgotPassword,
			common: translations.common,
		},
	});

	return (
		<Wrapper>
			<Text>{t("body")}</Text>

			<PrimaryButton href={url}>
				{t("resetPassword")} &rarr;
			</PrimaryButton>

			<Text className="text-muted-foreground text-sm">
				{t("common.openLinkInBrowser")}
				<Link href={url}>{url}</Link>
			</Text>
		</Wrapper>
	);
}

ForgotPassword.PreviewProps = {
	locale: defaultLocale,
	translations: defaultTranslations,
	url: "#",
	name: "John Doe",
};

export default ForgotPassword;
