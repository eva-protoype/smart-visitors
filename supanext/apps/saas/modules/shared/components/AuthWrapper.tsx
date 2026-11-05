import { config } from "@config";
import { cn, Logo } from "@repo/ui";
import type { PropsWithChildren } from "react";
import { ColorModeToggle } from "./ColorModeToggle";
import { Footer } from "./Footer";
import { LocaleSwitch } from "./LocaleSwitch";

export function AuthWrapper({
	children,
	contentClass,
}: PropsWithChildren<{ contentClass?: string }>) {
	return (
		<div className="flex min-h-screen w-full py-6">
			<div className="flex w-full flex-col items-center justify-between gap-8">
				<div className="container">
					<div className="flex items-center justify-between">
						<a href={config.marketingUrl ?? "/"} className="block">
							<Logo />
						</a>

						<div className="flex items-center justify-end gap-2">
							<LocaleSwitch />
							<ColorModeToggle />
						</div>
					</div>
				</div>

				<div className="container flex justify-center">
					<main
						className={cn(
							"w-full max-w-md rounded-3xl bg-card p-6 border lg:p-8",
							contentClass,
						)}
					>
						{children}
					</main>
				</div>

				<Footer />
			</div>
		</div>
	);
}
