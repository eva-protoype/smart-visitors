import { config } from "@repo/payments/config";
import { useTranslations } from "next-intl";
import type { ReactNode } from "react";

type PlanDataEntry = {
	title: string;
	description: ReactNode;
	features: ReactNode[];
};

export function usePlanData() {
	const t = useTranslations();

	const planData: Record<string, PlanDataEntry> = {};

	for (const planId of Object.keys(config.plans)) {
		planData[planId] = {
			title: t(`pricing.products.${planId}.title`),
			description: t(`pricing.products.${planId}.description`),
			features: Object.values(
				(t.raw(`pricing.products.${planId}.features`) as Record<
					string,
					string
				>) ?? {},
			),
		};
	}

	if (!config.requireActiveSubscription) {
		planData.free = {
			title: t("pricing.products.free.title"),
			description: t("pricing.products.free.description"),
			features: Object.values(
				(t.raw("pricing.products.free.features") as Record<
					string,
					string
				>) ?? {},
			),
		};
	}

	return { planData };
}
