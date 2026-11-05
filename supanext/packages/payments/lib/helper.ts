import type { PurchaseSchema } from "@repo/database";
import type { z } from "zod";
import { config } from "../config";
import type { PlanPrice } from "../types";
import type { PlanId } from "./plans";
import {
	getPlanIdByProviderPriceId,
	getPlanPriceByProviderPriceId,
} from "./provider-price-ids";

type PurchaseWithoutTimestamps = Omit<
	z.infer<typeof PurchaseSchema>,
	"createdAt" | "updatedAt"
>;

export interface ResolvedPurchase extends PurchaseWithoutTimestamps {
	planId?: PlanId | null;
	planPrice?: PlanPrice | null;
}

function resolvePurchasePlan(purchase: ResolvedPurchase) {
	if (purchase.planId && purchase.planPrice) {
		return {
			planId: purchase.planId,
			price: purchase.planPrice,
		};
	}

	const resolvedPrice = getPlanPriceByProviderPriceId(purchase.priceId);

	if (!resolvedPrice) {
		return null;
	}

	return resolvedPrice;
}

function resolvePurchasePlanId(purchase: ResolvedPurchase) {
	if (purchase.planId) {
		return purchase.planId;
	}

	return getPlanIdByProviderPriceId(purchase.priceId);
}

function getActivePlanFromPurchases(purchases?: ResolvedPurchase[]) {
	const subscriptionPurchase = purchases?.find(
		(purchase) => purchase.type === "SUBSCRIPTION",
	);

	if (subscriptionPurchase) {
		const resolvedPrice = resolvePurchasePlan(subscriptionPurchase);

		if (!resolvedPrice || !(resolvedPrice.planId in config.plans)) {
			return null;
		}

		return {
			id: resolvedPrice.planId,
			price: resolvedPrice.price,
			status: subscriptionPurchase.status || "active",
			purchaseId: subscriptionPurchase.id,
		};
	}

	const oneTimePurchase = purchases?.find(
		(purchase) => purchase.type === "ONE_TIME",
	);

	if (oneTimePurchase) {
		const resolvedPrice = resolvePurchasePlan(oneTimePurchase);

		if (!resolvedPrice || !(resolvedPrice.planId in config.plans)) {
			return null;
		}

		return {
			id: resolvedPrice.planId,
			price: resolvedPrice.price,
			status: "active",
			purchaseId: oneTimePurchase.id,
		};
	}

	if (!config.requireActiveSubscription) {
		return {
			id: "free" as PlanId,
			price: undefined,
			status: "active",
		};
	}

	return null;
}

export function createPurchasesHelper(purchases: ResolvedPurchase[]) {
	const activePlan = getActivePlanFromPurchases(purchases);

	const hasSubscription = (planIds?: PlanId[] | PlanId) => {
		return (
			!!activePlan &&
			(Array.isArray(planIds)
				? planIds.includes(activePlan.id)
				: planIds === activePlan.id)
		);
	};

	const hasPurchase = (planId: PlanId) => {
		return !!purchases?.some(
			(purchase) => resolvePurchasePlanId(purchase) === planId,
		);
	};

	return { activePlan, hasSubscription, hasPurchase };
}
