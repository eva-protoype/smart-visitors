import { getSession } from "@auth/lib/server";
import { config as paymentsConfig } from "@repo/payments/config";
import { SettingsMenu } from "@settings/components/SettingsMenu";
import { PageHeader } from "@shared/components/PageHeader";
import { UserAvatar } from "@shared/components/UserAvatar";
import {
	CreditCardIcon,
	LockKeyholeIcon,
	SettingsIcon,
	TriangleAlertIcon,
} from "lucide-react";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import type { PropsWithChildren } from "react";
export default async function SettingsLayout({ children }: PropsWithChildren) {
	const t = await getTranslations("settings");
	const session = await getSession();

	if (!session) {
		redirect("/login");
	}

	const menuItems = [
		{
			title: t("menu.account.title"),
			avatar: (
				<UserAvatar
					name={session.user.name ?? ""}
					avatarUrl={session.user.image}
				/>
			),
			items: [
				{
					title: t("menu.account.general"),
					href: "/settings/general",
					icon: <SettingsIcon className="size-4 opacity-50" />,
				},
				{
					title: t("menu.account.security"),
					href: "/settings/security",
					icon: <LockKeyholeIcon className="size-4 opacity-50" />,
				},
				...(paymentsConfig.billingAttachedTo === "user"
					? [
							{
								title: t("menu.account.billing"),
								href: "/settings/billing",
								icon: (
									<CreditCardIcon className="size-4 opacity-50" />
								),
							},
						]
					: []),
				{
					title: t("menu.account.dangerZone"),
					href: "/settings/danger-zone",
					icon: <TriangleAlertIcon className="size-4 opacity-50" />,
				},
			],
		},
	];

	return (
		<>
			<PageHeader
				title={t("account.title")}
				subtitle={t("account.subtitle")}
			/>

			<SettingsMenu menuItems={menuItems} className="mb-6" />

			{children}
		</>
	);
}
