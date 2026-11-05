import "server-only";
import { headers } from "next/headers";
import type { GoSession } from "./go-session-types";

const apiUrl = (
	process.env.API_URL ??
	process.env.BACKEND_URL ??
	"http://localhost:8000"
).replace(/\/$/, "");

export async function getGoSession(): Promise<GoSession | null> {
	const requestHeaders = await headers();
	const cookie = requestHeaders.get("cookie");
	const response = await fetch(`${apiUrl}/api/me`, {
		headers: cookie ? { cookie } : undefined,
		cache: "no-store",
	});

	if (response.status === 401) {
		return null;
	}
	if (!response.ok) {
		throw new Error(`Visitor API session lookup failed (${response.status}).`);
	}
	return (await response.json()) as GoSession;
}
