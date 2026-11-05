"use client";

import { useSession } from "@auth/hooks/use-session";
import { sessionQueryKey } from "@auth/lib/api";
import {
	activeOrganizationQueryKey,
	useActiveOrganizationQuery,
} from "@organizations/lib/api";
import { authClient } from "@repo/auth/client";
import { isOrganizationAdmin } from "@repo/auth/lib/helper";
import { config as paymentsConfig } from "@repo/payments/config";
import { orpc } from "@shared/lib/orpc-query-utils";
import { useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import nProgress from "nprogress";
import { type ReactNode, useEffect, useState } from "react";
import { ActiveOrganizationContext } from "../lib/active-organization-context";

export function ActiveOrganizationProvider({
	children,
}: {
	children: ReactNode;
}) {
	const queryClient = useQueryClient();
	const { session, user } = useSession();
	const params = useParams();

	const organizationSlug = params.organizationSlug as string | undefined;
	const activeOrganizationId = session?.activeOrganizationId;
	const activeOrganizationIdentifier = organizationSlug
		? { slug: organizationSlug }
		: activeOrganizationId
			? { id: activeOrganizationId }
			: { slug: undefined, id: undefined };

	const { data: activeOrganization } = useActiveOrganizationQuery(
		activeOrganizationIdentifier,
		{
			enabled: !!organizationSlug || !!activeOrganizationId,
		},
	);

	const refetchActiveOrganization = async () => {
		await queryClient.refetchQueries({
			queryKey: activeOrganizationQueryKey(activeOrganizationIdentifier),
		});
	};

	const setActiveOrganization = async (organizationSlug: string | null) => {
		nProgress.start();
		const { data: newActiveOrganization } =
			await authClient.organization.setActive(
				organizationSlug
					? {
							organizationSlug,
						}
					: {
							organizationId: null,
						},
			);

		const newActiveOrganizationId = newActiveOrganization?.id ?? null;

		await authClient.updateUser({
			lastActiveOrganizationId: newActiveOrganizationId,
		});

		await queryClient.setQueryData(sessionQueryKey, (data: any) => {
			return {
				...data,
				session: {
					...data?.session,
					activeOrganizationId: newActiveOrganizationId,
				},
				user: {
					...data?.user,
					lastActiveOrganizationId: newActiveOrganizationId,
				},
			};
		});

		if (!newActiveOrganization) {
			nProgress.done();
			return;
		}

		await refetchActiveOrganization();

		if (paymentsConfig.billingAttachedTo === "organization") {
			await queryClient.prefetchQuery(
				orpc.payments.listPurchases.queryOptions({
					input: {
						organizationId: newActiveOrganization.id,
					},
				}),
			);
		}
	};

	const [loaded, setLoaded] = useState(activeOrganization !== undefined);

	useEffect(() => {
		if (!loaded && activeOrganization !== undefined) {
			setLoaded(true);
		}
	}, [activeOrganization]);

	const activeOrganizationUserRole = activeOrganization?.members.find(
		(member) => member.userId === session?.userId,
	)?.role;

	return (
		<ActiveOrganizationContext.Provider
			value={{
				loaded,
				activeOrganization: activeOrganization ?? null,
				activeOrganizationUserRole: activeOrganizationUserRole ?? null,
				isOrganizationAdmin:
					!!activeOrganization &&
					!!user &&
					isOrganizationAdmin(activeOrganization, user),
				setActiveOrganization,
				refetchActiveOrganization,
			}}
		>
			{children}
		</ActiveOrganizationContext.Provider>
	);
}
