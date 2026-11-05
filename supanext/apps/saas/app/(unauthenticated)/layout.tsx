import { AuthWrapper } from "@shared/components/AuthWrapper";
import type { PropsWithChildren } from "react";

export default async function AuthLayout({ children }: PropsWithChildren) {
	return <AuthWrapper>{children}</AuthWrapper>;
}
