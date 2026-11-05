import { SessionProvider } from "@auth/components/SessionProvider";
import { AuthWrapper } from "@shared/components/AuthWrapper";
import type { PropsWithChildren } from "react";

export default async function AuthLayout({ children }: PropsWithChildren) {
	return (
		<SessionProvider>
			<AuthWrapper>{children}</AuthWrapper>
		</SessionProvider>
	);
}
