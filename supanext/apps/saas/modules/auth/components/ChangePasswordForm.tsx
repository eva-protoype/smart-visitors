"use client";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";

interface ApiError {
	error?: {
		message?: string;
	};
}

export function ChangePasswordForm() {
	const router = useRouter();
	const [error, setError] = useState("");
	const [submitting, setSubmitting] = useState(false);

	async function onSubmit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setSubmitting(true);

		const formData = new FormData(event.currentTarget);
		const newPassword = String(formData.get("newPassword") ?? "");
		if (newPassword !== formData.get("confirmPassword")) {
			setError("The new passwords do not match.");
			setSubmitting(false);
			return;
		}

		try {
			const response = await fetch("/api/me/password", {
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					old: formData.get("oldPassword"),
					new: newPassword,
				}),
			});
			if (!response.ok) {
				const result = (await response.json()) as ApiError;
				setError(result.error?.message ?? "Could not change password.");
				return;
			}
			router.replace("/login?passwordChanged=1");
			router.refresh();
		} catch {
			setError("The visitor service could not be reached.");
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<div>
			<h1 className="font-bold text-xl md:text-2xl">Set a new password</h1>
			<p className="mt-1 mb-6 text-foreground/60">
				Choose a new password to continue.
			</p>
			<form className="space-y-4" onSubmit={onSubmit}>
				<label className="block space-y-2 text-sm font-medium">
					Current password
					<Input
						name="oldPassword"
						type="password"
						autoComplete="current-password"
						required
					/>
				</label>
				<label className="block space-y-2 text-sm font-medium">
					New password
					<Input
						name="newPassword"
						type="password"
						autoComplete="new-password"
						minLength={12}
						required
					/>
				</label>
				<label className="block space-y-2 text-sm font-medium">
					Confirm new password
					<Input
						name="confirmPassword"
						type="password"
						autoComplete="new-password"
						minLength={12}
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
					Update password
				</Button>
			</form>
		</div>
	);
}
