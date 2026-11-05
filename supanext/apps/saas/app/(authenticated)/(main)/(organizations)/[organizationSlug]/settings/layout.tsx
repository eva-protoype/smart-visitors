import { getActiveOrganization, getSession } from "@auth/lib/server";
import { OrganizationLogo } from "@organizations/components/OrganizationLogo";
import { config as authConfig } from "@repo/auth/config";
import { isOrganizationAdmin } from "@repo/auth/lib/helper";
import { config as paymentsConfig } from "@repo/payments/config";
import { SettingsMenu } from "@settings/components/SettingsMenu";
import { PageHeader } from "@shared/components/PageHeader";
import {
	CreditCardIcon,
	Settings2Icon,
	TriangleAlertIcon,
	Users2Icon,
} from "lucide-react";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import type { PropsWithChildren } from "react";

export default async function SettingsLayout({
	children,
	params,
}: PropsWithChildren<{
	params: Promise<{ organizationSlug: string }>;
}>) {
	const t = await getTranslations("settings");
	const tOrg = await getTranslations("organizations.settings");
	const session = await getSession();
	const { organizationSlug } = await params;
	const organization = await getActiveOrganization(organizationSlug);

	if (!organization) {
		redirect("/");
	}

	const userIsOrganizationAdmin = isOrganizationAdmin(
		organization,
		session?.user,
	);

	const organizationSettingsBasePath = `/${organizationSlug}/settings`;

	const menuItems = [
		{
			title: t("menu.organization.title"),
			avatar: (
				<OrganizationLogo
					name={organization.name}
					logoUrl={organization.logo}
				/>
			),
			items: [
				{
					title: t("menu.organization.general"),
					href: `${organizationSettingsBasePath}/general`,
					icon: <Settings2Icon className="size-4 opacity-50" />,
				},
				{
					title: t("menu.organization.members"),
					href: `${organizationSettingsBasePath}/members`,
					icon: <Users2Icon className="size-4 opacity-50" />,
				},
				...(authConfig.organizations.enable &&
				paymentsConfig.billingAttachedTo === "organization" &&
				userIsOrganizationAdmin
					? [
							{
								title: t("menu.organization.billing"),
								href: `${organizationSettingsBasePath}/billing`,
								icon: (
									<CreditCardIcon className="size-4 opacity-50" />
								),
							},
						]
					: []),
				...(userIsOrganizationAdmin
					? [
							{
								title: t("menu.organization.dangerZone"),
								href: `${organizationSettingsBasePath}/danger-zone`,
								icon: (
									<TriangleAlertIcon className="size-4 opacity-50" />
								),
							},
						]
					: []),
			],
		},
	];

	return (
		<>
			<PageHeader title={tOrg("title")} subtitle={tOrg("subtitle")} />

			<SettingsMenu menuItems={menuItems} className="mb-6" />

			{children}
		</>
	);
}
