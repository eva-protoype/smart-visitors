import { getGoSession } from "@auth/lib/go-session";
import { redirect } from "next/navigation";
import type { PropsWithChildren } from "react";

export default async function AdminLayout({ children }: PropsWithChildren) {
	const session = await getGoSession();

	if (!session || session.user.role !== "admin") {
		redirect("/");
	}

	return children;
}
