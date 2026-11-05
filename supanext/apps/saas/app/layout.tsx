import { cn, Toaster } from "@repo/ui";
import { ApiClientProvider } from "@shared/components/ApiClientProvider";
import type { Metadata } from "next";
import { Figtree } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getMessages } from "next-intl/server";
import { ThemeProvider } from "next-themes";
import { NuqsAdapter } from "nuqs/adapters/next/app";
import type { PropsWithChildren } from "react";
import "./globals.css";
import "cropperjs/dist/cropper.css";
import { config } from "@config";
import { ClientProviders } from "@shared/components/ClientProviders";

const sansFont = Figtree({
	weight: ["300", "400", "500", "600", "700"],
	subsets: ["latin"],
	variable: "--font-sans",
});

export const metadata: Metadata = {
	title: {
		absolute: config.appName,
		default: config.appName,
		template: `%s | ${config.appName}`,
	},
};

export default async function RootLayout({ children }: PropsWithChildren) {
	const locale = await getLocale();
	const messages = await getMessages();

	return (
		<html
			lang={locale}
			suppressHydrationWarning
			className={sansFont.variable}
		>
			<body
				className={cn(
					"min-h-screen bg-background text-foreground antialiased",
				)}
			>
				<NuqsAdapter>
					<NextIntlClientProvider messages={messages}>
						<ThemeProvider
							attribute="class"
							disableTransitionOnChange
							enableSystem
							defaultTheme={config.defaultTheme}
							themes={Array.from(config.enabledThemes)}
						>
							<ApiClientProvider>
								<ClientProviders>
									{children}

									<Toaster position="top-right" />
								</ClientProviders>
							</ApiClientProvider>
						</ThemeProvider>
					</NextIntlClientProvider>
				</NuqsAdapter>
			</body>
		</html>
	);
}
