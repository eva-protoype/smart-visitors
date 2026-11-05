import { getSession } from "@auth/lib/server";
import { ChangeEmailForm } from "@settings/components/ChangeEmailForm";
import { ChangeNameForm } from "@settings/components/ChangeNameForm";
import { UserAvatarForm } from "@settings/components/UserAvatarForm";
import { UserLanguageForm } from "@settings/components/UserLanguageForm";
import { SettingsList } from "@shared/components/SettingsList";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

export async function generateMetadata() {
	const t = await getTranslations("settings.account");

	return {
		title: t("title"),
	};
}

export default async function AccountSettingsPage() {
	const session = await getSession();

	if (!session) {
		redirect("/login");
	}

	return (
		<SettingsList>
			<UserAvatarForm />
			<UserLanguageForm />
			<ChangeNameForm />
			<ChangeEmailForm />
		</SettingsList>
	);
}
