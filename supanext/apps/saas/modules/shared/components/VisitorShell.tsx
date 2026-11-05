"use client";

import type { GoUser } from "@auth/lib/go-session-types";
import { Button } from "@repo/ui/components/button";
import { LogOut, ShieldCheck } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type PropsWithChildren } from "react";

export function VisitorShell({
	children,
	user,
}: PropsWithChildren<{ user: GoUser }>) {
	const router = useRouter();
	const [error, setError] = useState("");

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
			<header className="border-b border-border bg-card/80">
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
						{user.role === "admin" && (
							<Link
								href="/admin/users"
								className="hidden text-sm font-medium text-muted-foreground hover:text-foreground sm:inline"
							>
								Team accounts
							</Link>
						)}
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
