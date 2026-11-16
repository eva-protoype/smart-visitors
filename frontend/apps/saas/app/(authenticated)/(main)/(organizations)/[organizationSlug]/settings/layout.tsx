import { redirect } from "next/navigation";
import type { PropsWithChildren } from "react";

export default function OrganizationSettingsLayout({
	children,
}: PropsWithChildren) {
	void children;
	redirect("/");
}
