import { redirect } from "next/navigation";
import type { PropsWithChildren } from "react";

export default function SettingsLayout({ children }: PropsWithChildren) {
	void children;
	redirect("/");
}
