import { listPurchases as listPurchasesProcedure } from "@repo/api/modules/payments/procedures/list-purchases";
import { headers } from "next/headers";
import { cache } from "react";

export const listPurchases = cache(async (organizationId?: string) => {
	const purchases = await listPurchasesProcedure.callable({
		context: { headers: await headers() },
	})({
		organizationId,
	});

	return purchases;
});
