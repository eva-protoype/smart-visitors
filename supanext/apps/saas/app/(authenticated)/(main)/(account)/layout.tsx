import { getGoSession } from "@auth/lib/go-session";
import { VisitorShell } from "@shared/components/VisitorShell";
import { redirect } from "next/navigation";
import type { PropsWithChildren } from "react";

export default async function AccountLayout({
	children,
}: PropsWithChildren) {
	const session = await getGoSession();

	if (!session) {
		redirect("/login");
	}

	return <VisitorShell user={session.user}>{children}</VisitorShell>;
}
