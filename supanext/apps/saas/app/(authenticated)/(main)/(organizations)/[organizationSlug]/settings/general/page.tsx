import { ChangeOrganizationNameForm } from "@organizations/components/ChangeOrganizationNameForm";
import { OrganizationLogoForm } from "@organizations/components/OrganizationLogoForm";
import { SettingsList } from "@shared/components/SettingsList";
import { getTranslations } from "next-intl/server";

export async function generateMetadata() {
	const t = await getTranslations("organizations.settings");

	return {
		title: t("title"),
	};
}

export default function OrganizationSettingsPage() {
	return (
		<SettingsList>
			<OrganizationLogoForm />
			<ChangeOrganizationNameForm />
		</SettingsList>
	);
}
