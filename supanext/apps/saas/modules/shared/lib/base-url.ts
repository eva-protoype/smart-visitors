import { getBaseUrl as getBaseUrlFromUtils } from "@repo/utils";

export function getBaseUrl() {
	return getBaseUrlFromUtils(process.env.NEXT_PUBLIC_SAAS_URL, 3000);
}
