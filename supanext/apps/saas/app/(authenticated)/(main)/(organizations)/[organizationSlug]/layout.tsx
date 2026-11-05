import { getActiveOrganization } from "@auth/lib/server";
import { activeOrganizationQueryKey } from "@organizations/lib/api";
import { listPurchases } from "@payments/lib/server";
import { config as paymentsConfig } from "@repo/payments/config";
import { AppWrapper } from "@shared/components/AppWrapper";
import { orpc } from "@shared/lib/orpc-query-utils";
import { getServerQueryClient } from "@shared/lib/server";
import { notFound } from "next/navigation";
import type { PropsWithChildren } from "react";

export default async function OrganizationLayout({
	children,
	params,
}: PropsWithChildren<{
	params: Promise<{
		organizationSlug: string;
	}>;
}>) {
	const { organizationSlug } = await params;

	const organization = await getActiveOrganization(organizationSlug);

	if (!organization) {
		return notFound();
	}

	const queryClient = getServerQueryClient();

	await queryClient.prefetchQuery({
		queryKey: activeOrganizationQueryKey({ slug: organizationSlug }),
		queryFn: () => organization,
	});

	if (paymentsConfig.billingAttachedTo === "organization") {
		await queryClient.prefetchQuery({
			queryKey: orpc.payments.listPurchases.queryKey({
				input: {
					organizationId: organization.id,
				},
			}),
			queryFn: () => listPurchases(organization.id),
		});
	}

	return <AppWrapper>{children}</AppWrapper>;
}
