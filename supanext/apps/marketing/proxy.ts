import { routing } from "@i18n/routing";
import type { NextRequest } from "next/server";
import createMiddleware from "next-intl/middleware";

const intlMiddleware = createMiddleware(routing);

export default async function proxy(req: NextRequest) {
	return intlMiddleware(req);
}

export const config = {
	matcher: [
		"/((?!images|fonts|_next/static|_next/image|favicon.ico|icon.png|sitemap.xml|robots.txt).*)",
	],
};
