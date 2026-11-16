import { getGoSession } from "@auth/lib/go-session";
import { redirect } from "next/navigation";
import type { PropsWithChildren } from "react";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export default async function AuthenticatedLayout({
	children,
}: PropsWithChildren) {
	const session = await getGoSession();

	if (!session) {
		redirect("/login");
	}
	if (session.user.must_reset_password) {
		redirect("/change-password");
	}

	return children;
}
