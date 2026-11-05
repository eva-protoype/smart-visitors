import { getOrganizationList, getSession } from "@auth/lib/server";
import { PricingTable } from "@payments/components/PricingTable";
import { listPurchases } from "@payments/lib/server";
import { config as authConfig } from "@repo/auth/config";
import { config as paymentsConfig } from "@repo/payments/config";
import { createPurchasesHelper } from "@repo/payments/lib/helper";
import { AuthWrapper } from "@shared/components/AuthWrapper";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export async function generateMetadata() {
	const t = await getTranslations("choosePlan");

	return {
		title: t("title"),
	};
}

export default async function ChoosePlanPage() {
	const t = await getTranslations("choosePlan");
	const session = await getSession();

	if (!session) {
		redirect("/login");
	}

	let organizationId: string | undefined;
	if (
		authConfig.organizations.enable &&
		paymentsConfig.billingAttachedTo === "organization"
	) {
		const organization = (await getOrganizationList()).at(0);

		if (!organization) {
			redirect("/new-organization");
		}

		organizationId = organization.id;
	}

	const purchases = await listPurchases(organizationId);

	const { activePlan } = createPurchasesHelper(purchases);

	if (activePlan) {
		redirect("/");
	}

	return (
		<AuthWrapper contentClass="max-w-5xl">
			<div className="mb-4 text-center">
				<h1 className="text-center font-bold text-2xl lg:text-3xl">
					{t("title")}
				</h1>
				<p className="text-muted-foreground text-sm lg:text-base">
					{t("description")}
				</p>
			</div>

			<div>
				<PricingTable
					{...(organizationId
						? {
								organizationId,
							}
						: {
								userId: session.user.id,
							})}
				/>
			</div>
		</AuthWrapper>
	);
}
