import type { SaasConfig } from "./types";

export const config = {
	appName: "Smart Visitors",
	docsUrl: process.env.NEXT_PUBLIC_DOCS_URL as string | undefined,
	marketingUrl: process.env.NEXT_PUBLIC_MARKETING_URL as string | undefined,
	enabledThemes: ["light", "dark"],
	defaultTheme: "light",
	useSidebarLayout: true,
	redirectAfterSignIn: "/",
	redirectAfterLogout: "/login",
} as const satisfies SaasConfig;
