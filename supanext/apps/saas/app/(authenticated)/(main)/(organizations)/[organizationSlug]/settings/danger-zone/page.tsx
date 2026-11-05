import { DeleteOrganizationForm } from "@organizations/components/DeleteOrganizationForm";
import { SettingsList } from "@shared/components/SettingsList";
import { getTranslations } from "next-intl/server";

export async function generateMetadata() {
	const t = await getTranslations("organizations.settings.dangerZone");

	return {
		title: t("title"),
	};
}

export default function OrganizationSettingsPage() {
	return (
		<SettingsList>
			<DeleteOrganizationForm />
		</SettingsList>
	);
}
