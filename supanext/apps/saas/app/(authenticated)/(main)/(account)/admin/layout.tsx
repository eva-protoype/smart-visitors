import { getSession } from "@auth/lib/server";
import { config } from "@repo/auth/config";
import { Logo } from "@repo/ui";
import { SettingsMenu } from "@settings/components/SettingsMenu";
import { PageHeader } from "@shared/components/PageHeader";
import { Building2Icon, UsersIcon } from "lucide-react";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import type { PropsWithChildren } from "react";

export default async function AdminLayout({ children }: PropsWithChildren) {
	const t = await getTranslations("admin");
	const session = await getSession();

	if (!session) {
		redirect("/login");
	}

	if (session.user?.role !== "admin") {
		redirect("/");
	}

	return (
		<>
			<PageHeader title={t("title")} subtitle={t("description")} />

			<SettingsMenu
				className="mb-6"
				menuItems={[
					{
						avatar: <Logo className="size-8" withLabel={false} />,
						title: t("title"),
						items: [
							{
								title: t("menu.users"),
								href: "/admin/users",
								icon: (
									<UsersIcon className="size-4 opacity-50" />
								),
							},
							...(config.organizations.enable
								? [
										{
											title: t("menu.organizations"),
											href: "/admin/organizations",
											icon: (
												<Building2Icon className="size-4 opacity-50" />
											),
										},
									]
								: []),
						],
					},
				]}
			/>

			{children}
		</>
	);
}
