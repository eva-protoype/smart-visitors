"use client";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { useRouter } from "next/navigation";
import { useSearchParams } from "next/navigation";
import { useState, type FormEvent } from "react";

interface LoginResponse {
	user?: {
		must_reset_password: boolean;
	};
	error?: {
		message?: string;
	};
}

export function GoLoginForm() {
	const router = useRouter();
	const searchParams = useSearchParams();
	const [error, setError] = useState("");
	const [submitting, setSubmitting] = useState(false);

	async function onSubmit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setSubmitting(true);

		const formData = new FormData(event.currentTarget);
		try {
			const response = await fetch("/api/auth/login", {
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					username: formData.get("username"),
					password: formData.get("password"),
				}),
			});
			const result = (await response.json()) as LoginResponse;
			if (!response.ok || !result.user) {
				setError(result.error?.message ?? "Unable to sign in.");
				return;
			}

			if (result.user.must_reset_password) {
				router.replace("/change-password");
				return;
			}

			const destination = searchParams.get("redirectTo");
			router.replace(
				destination?.startsWith("/") && !destination.startsWith("//")
					? destination
					: "/",
			);
		} catch {
			setError("The visitor service could not be reached.");
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<div>
			<h1 className="font-bold text-xl md:text-2xl">Welcome back</h1>
			<p className="mt-1 mb-6 text-foreground/60">
				Sign in to manage visitor access.
			</p>
			<form className="space-y-4" onSubmit={onSubmit}>
				<label className="block space-y-2 text-sm font-medium">
					Username
					<Input
						name="username"
						autoComplete="username"
						required
						maxLength={100}
					/>
				</label>
				<label className="block space-y-2 text-sm font-medium">
					Password
					<Input
						name="password"
						type="password"
						autoComplete="current-password"
						required
					/>
				</label>
				{error && (
					<p className="text-sm text-destructive" role="alert">
						{error}
					</p>
				)}
				<Button
					className="w-full"
					type="submit"
					variant="primary"
					loading={submitting}
				>
					Sign in
				</Button>
			</form>
		</div>
	);
}
