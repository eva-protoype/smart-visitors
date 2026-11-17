"use client";

import type { GoUser } from "@auth/lib/go-session-types";
import { Button } from "@repo/ui/components/button";
import {
	BadgeCheck,
	ClipboardList,
	LogOut,
	ScanLine,
	ShieldAlert,
	ShieldCheck,
	UsersRound,
	type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { usePathname } from "next/navigation";
import { useState, type PropsWithChildren } from "react";

interface NavigationItem {
	href: string;
	label: string;
	icon: LucideIcon;
}

const NAVIGATION: Record<GoUser["role"], NavigationItem[]> = {
	admin: [
		{ href: "/admin", label: "Overview", icon: ShieldCheck },
		{ href: "/admin/users", label: "Team accounts", icon: UsersRound },
		{ href: "/admin#activity", label: "Gate activity", icon: ClipboardList },
		{ href: "/admin#security", label: "Security watch", icon: ShieldAlert },
	],
	host: [
		{ href: "/host", label: "My workspace", icon: BadgeCheck },
		{ href: "/host#host-scan", label: "Verify pass", icon: ShieldCheck },
		{ href: "/host#visits", label: "My visits", icon: UsersRound },
	],
	guard: [
		{ href: "/guard", label: "Gate console", icon: ScanLine },
		{ href: "/scanner", label: "Scan pass", icon: ShieldCheck },
		{ href: "/guard#walk-in", label: "Walk-in", icon: ClipboardList },
	],
};

export function VisitorShell({
	children,
	user,
}: PropsWithChildren<{ user: GoUser }>) {
	const router = useRouter();
	const pathname = usePathname();
	const [error, setError] = useState("");
	const navigation = NAVIGATION[user.role];

	async function signOut() {
		setError("");
		try {
			const response = await fetch("/api/auth/logout", {
				method: "POST",
				credentials: "same-origin",
			});
			if (!response.ok) {
				setError("Could not sign out. Please try again.");
				return;
			}
			router.replace("/login");
			router.refresh();
		} catch {
			setError("The visitor service could not be reached.");
		}
	}

	return (
		<div className="min-h-screen bg-background text-foreground">
			<header className="border-b border-border bg-card/95">
				<div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-4 sm:px-6 lg:px-8">
					<Link href="/" className="flex items-center gap-2">
						<span className="flex size-9 items-center justify-center rounded-xl bg-[#eaf0e9] text-[#587662]">
							<ShieldCheck className="size-5" aria-hidden="true" />
						</span>
						<span className="font-semibold tracking-tight">
							Smart Visitors
						</span>
					</Link>
					<div className="flex items-center gap-3">
						<div className="hidden text-right sm:block">
							<p className="text-sm font-medium">{user.name}</p>
							<p className="text-xs capitalize text-muted-foreground">
								{user.role}
							</p>
						</div>
						<Button
							type="button"
							variant="secondary"
							onClick={signOut}
							aria-label="Sign out"
						>
							<LogOut className="size-4" aria-hidden="true" />
							<span className="hidden sm:inline">Sign out</span>
						</Button>
					</div>
				</div>
				<nav
					className="mx-auto flex max-w-7xl gap-1 overflow-x-auto px-4 pb-3 sm:px-6 lg:px-8"
					aria-label={`${user.role} navigation`}
				>
					{navigation.map((item) => {
						const Icon = item.icon;
						const isActive =
							item.href === "/"
								? pathname === "/"
								: pathname === item.href ||
									pathname.startsWith(`${item.href}/`);

						return (
							<Link
								key={item.href}
								href={item.href}
								className={`inline-flex shrink-0 items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition-colors ${
									isActive
										? "bg-[#eaf0e9] text-[#44634f]"
										: "text-muted-foreground hover:bg-muted hover:text-foreground"
								}`}
								aria-current={isActive ? "page" : undefined}
							>
								<Icon className="size-4" aria-hidden="true" />
								{item.label}
							</Link>
						);
					})}
				</nav>
			</header>
			{error && (
				<p
					className="mx-auto mt-4 max-w-7xl px-4 text-sm text-destructive sm:px-6 lg:px-8"
					role="alert"
				>
					{error}
				</p>
			)}
			<main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
				{children}
			</main>
		</div>
	);
}
