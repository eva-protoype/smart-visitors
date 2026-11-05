import { getActiveOrganization } from "@auth/lib/server";
import { ActivePlan } from "@payments/components/ActivePlan";
import { ChangePlan } from "@payments/components/ChangePlan";
import { listPurchases } from "@payments/lib/server";
import { createPurchasesHelper } from "@repo/payments/lib/helper";
import { SettingsList } from "@shared/components/SettingsList";
import { orpc } from "@shared/lib/orpc-query-utils";
import { getServerQueryClient } from "@shared/lib/server";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";

export async function generateMetadata() {
	const t = await getTranslations("settings.billing");

	return {
		title: t("title"),
	};
}

export default async function BillingSettingsPage({
	params,
}: {
	params: Promise<{ organizationSlug: string }>;
}) {
	const { organizationSlug } = await params;
	const organization = await getActiveOrganization(organizationSlug);

	if (!organization) {
		return notFound();
	}

	const purchases = await listPurchases(organization.id);

	const queryClient = getServerQueryClient();

	await queryClient.prefetchQuery({
		queryKey: orpc.payments.listPurchases.queryKey({
			input: {
				organizationId: organization.id,
			},
		}),
		queryFn: () => purchases,
	});

	const { activePlan } = createPurchasesHelper(purchases);

	return (
		<SettingsList>
			{activePlan && <ActivePlan organizationId={organization.id} />}
			<ChangePlan
				organizationId={organization.id}
				activePlanId={activePlan?.id}
			/>
		</SettingsList>
	);
}
