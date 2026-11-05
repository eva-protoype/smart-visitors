import { SessionProvider } from "@auth/components/SessionProvider";
import { sessionQueryKey } from "@auth/lib/api";
import { getOrganizationList, getSession } from "@auth/lib/server";
import { ActiveOrganizationProvider } from "@organizations/components/ActiveOrganizationProvider";
import { organizationListQueryKey } from "@organizations/lib/api";
import { listPurchases } from "@payments/lib/server";
import { config as authConfig } from "@repo/auth/config";
import { config as paymentsConfig } from "@repo/payments/config";
import { ConfirmationAlertProvider } from "@shared/components/ConfirmationAlertProvider";
import { orpc } from "@shared/lib/orpc-query-utils";
import { getServerQueryClient } from "@shared/lib/server";
import { dehydrate, HydrationBoundary } from "@tanstack/react-query";
import { redirect } from "next/navigation";
import type { PropsWithChildren } from "react";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export default async function AuthenticatedLayout({
	children,
}: PropsWithChildren) {
	const session = await getSession();

	if (!session) {
		redirect("/login");
	}

	const queryClient = getServerQueryClient();

	await queryClient.prefetchQuery({
		queryKey: sessionQueryKey,
		queryFn: () => session,
	});

	if (authConfig.organizations.enable) {
		await queryClient.prefetchQuery({
			queryKey: organizationListQueryKey,
			queryFn: getOrganizationList,
		});
	}

	if (paymentsConfig.billingAttachedTo === "user") {
		await queryClient.prefetchQuery({
			queryKey: orpc.payments.listPurchases.queryKey({
				input: {},
			}),
			queryFn: () => listPurchases(),
		});
	}

	return (
		<HydrationBoundary state={dehydrate(queryClient)}>
			<SessionProvider>
				<ActiveOrganizationProvider>
					<ConfirmationAlertProvider>
						{children}
					</ConfirmationAlertProvider>
				</ActiveOrganizationProvider>
			</SessionProvider>
		</HydrationBoundary>
	);
}
