import { redirect } from "next/navigation";
import type { PropsWithChildren } from "react";

export default function OrganizationLayout({ children }: PropsWithChildren) {
	void children;
	redirect("/");
}
