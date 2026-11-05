import { getSession } from "@auth/lib/server";
import { DeleteAccountForm } from "@settings/components/DeleteAccountForm";
import { SettingsList } from "@shared/components/SettingsList";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

export async function generateMetadata() {
	const t = await getTranslations("settings.menu.account");

	return {
		title: t("dangerZone"),
	};
}

export default async function AccountSettingsPage() {
	const session = await getSession();

	if (!session) {
		redirect("/login");
	}

	return (
		<SettingsList>
			<DeleteAccountForm />
		</SettingsList>
	);
}
