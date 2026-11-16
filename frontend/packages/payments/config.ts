import type { PaymentsConfig } from "./types";

export const config: PaymentsConfig = {
	billingAttachedTo: "user",
	requireActiveSubscription: false,
	plans: {
		pro: {
			recommended: true,
			prices: [
				{
					type: "subscription",
					priceId: process.env.PRICE_ID_PRO_MONTHLY as string,
					interval: "month",
					amount: 29,
					currency: "USD",
					seatBased: true,
					trialPeriodDays: 7,
				},
				{
					type: "subscription",
					priceId: process.env.PRICE_ID_PRO_YEARLY as string,
					interval: "year",
					amount: 290,
					currency: "USD",
					seatBased: true,
					trialPeriodDays: 7,
				},
			],
		},
		lifetime: {
			prices: [
				{
					type: "one-time",
					priceId: process.env.PRICE_ID_LIFETIME as string,
					amount: 799,
					currency: "USD",
				},
			],
		},
		enterprise: {
			isEnterprise: true,
		},
	},
};
