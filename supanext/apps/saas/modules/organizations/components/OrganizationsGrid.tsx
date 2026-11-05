"use client";

import { OrganizationLogo } from "@organizations/components/OrganizationLogo";
import { useActiveOrganization } from "@organizations/hooks/use-active-organization";
import { useOrganizationListQuery } from "@organizations/lib/api";
import { config } from "@repo/auth/config";
import { Card } from "@repo/ui/components/card";
import { ChevronRightIcon, PlusCircleIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

export function OrganizationsGrid() {
	const t = useTranslations();
	const router = useRouter();

	const { setActiveOrganization } = useActiveOrganization();
	const { data: allOrganizations } = useOrganizationListQuery();

	const handleClick = (organizationSlug: string) => {
		setActiveOrganization(organizationSlug);
		router.replace(`/${organizationSlug}`);
	};

	return (
		<div className="@container">
			<h2 className="mb-2 font-semibold text-lg">
				{t("organizations.organizationsGrid.title")}
			</h2>
			<div className="grid @2xl:grid-cols-3 @lg:grid-cols-2 grid-cols-1 gap-4">
				{allOrganizations?.map((organization) => (
					<Card
						key={organization.id}
						className="flex cursor-pointer items-center gap-4 overflow-hidden p-4"
						onClick={() => handleClick(organization.slug)}
					>
						<OrganizationLogo
							name={organization.name}
							logoUrl={organization.logo}
							className="size-12"
						/>
						<span className="flex items-center gap-1 text-base leading-tight">
							<span className="block font-medium">
								{organization.name}
							</span>
							<ChevronRightIcon className="size-4" />
						</span>
					</Card>
				))}

				{config.organizations.enableUsersToCreateOrganizations && (
					<Link
						href="/new-organization"
						className="flex h-full items-center justify-center gap-2 rounded-2xl bg-primary/5 p-4 text-primary transition-colors duration-150 hover:bg-primary/10"
					>
						<PlusCircleIcon />
						<span className="font-medium text-sm">
							{t(
								"organizations.organizationsGrid.createNewOrganization",
							)}
						</span>
					</Link>
				)}
			</div>
		</div>
	);
}
