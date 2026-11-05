import { getOrganizationList } from "@auth/lib/server";
import { CreateOrganizationForm } from "@organizations/components/CreateOrganizationForm";
import { config } from "@repo/auth/config";
import { AuthWrapper } from "@shared/components/AuthWrapper";
import { redirect } from "next/navigation";

export const dynamic = "force-dynamic";

export default async function NewOrganizationPage() {
	const organizations = await getOrganizationList();

	if (
		!config.organizations.enable ||
		(!config.organizations.enableUsersToCreateOrganizations &&
			(!config.organizations.requireOrganization ||
				organizations.length > 0))
	) {
		redirect("/");
	}

	return (
		<AuthWrapper>
			<CreateOrganizationForm />
		</AuthWrapper>
	);
}
