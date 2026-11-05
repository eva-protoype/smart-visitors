"use client";

import { useActiveOrganization } from "@organizations/hooks/use-active-organization";
import { useOrganizationListQuery } from "@organizations/lib/api";
import { authClient } from "@repo/auth/client";
import { Button } from "@repo/ui/components/button";
import { toastError, toastSuccess } from "@repo/ui/components/toast";
import { useConfirmationAlert } from "@shared/components/ConfirmationAlertProvider";
import { SettingsItem } from "@shared/components/SettingsItem";
import { useRouter } from "@shared/hooks/router";
import { useTranslations } from "next-intl";

export function DeleteOrganizationForm() {
	const t = useTranslations();
	const router = useRouter();
	const { confirm } = useConfirmationAlert();
	const { refetch: reloadOrganizations } = useOrganizationListQuery();
	const { activeOrganization, setActiveOrganization } =
		useActiveOrganization();

	if (!activeOrganization) {
		return null;
	}

	const handleDelete = async () => {
		confirm({
			title: t("organizations.settings.deleteOrganization.title"),
			message: t(
				"organizations.settings.deleteOrganization.confirmation",
			),
			destructive: true,
			onConfirm: async () => {
				const { error } = await authClient.organization.delete({
					organizationId: activeOrganization.id,
				});

				if (error) {
					toastError(
						t(
							"organizations.settings.notifications.organizationNotDeleted",
						),
					);
					return;
				}

				toastSuccess(
					t(
						"organizations.settings.notifications.organizationDeleted",
					),
				);
				await setActiveOrganization(null);
				await reloadOrganizations();
				router.replace("/");
			},
		});
	};

	return (
		<SettingsItem
			danger
			title={t("organizations.settings.deleteOrganization.title")}
			description={t(
				"organizations.settings.deleteOrganization.description",
			)}
		>
			<div className="mt-4 flex justify-end">
				<Button variant="destructive" onClick={handleDelete}>
					{t("organizations.settings.deleteOrganization.submit")}
				</Button>
			</div>
		</SettingsItem>
	);
}
