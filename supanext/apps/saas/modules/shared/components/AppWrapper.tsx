"use client";

import { config } from "@config";
import { cn } from "@repo/ui";
import type { PropsWithChildren } from "react";
import { SidebarProvider, useSidebar } from "../lib/sidebar-context";
import { NavBar } from "./NavBar";

function AppContent({ children }: PropsWithChildren) {
	const { isCollapsed } = useSidebar();
	const { useSidebarLayout } = config;

	return (
		<div className="bg-background">
			<NavBar />
			<div
				className={cn("flex min-h-screen", {
					"md:ml-[280px]": useSidebarLayout && !isCollapsed,
					"md:ml-[80px]": useSidebarLayout && isCollapsed,
				})}
			>
				<main
					className={cn(
						"bg-background py-4 w-full border-t md:border-t-0 md:border-l",
					)}
				>
					<div className="container h-full">{children}</div>
				</main>
			</div>
		</div>
	);
}

export function AppWrapper({ children }: PropsWithChildren) {
	return (
		<SidebarProvider>
			<AppContent>{children}</AppContent>
		</SidebarProvider>
	);
}
