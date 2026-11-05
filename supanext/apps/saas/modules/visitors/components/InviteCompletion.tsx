"use client";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { useEffect, useState, type FormEvent } from "react";

interface ApiError {
	error?: { message?: string };
}

interface Invite {
	pass_id: string;
	visitor: { name: string; phone: string | null };
	purpose: string;
	host_name: string;
}

interface ActivatedPass {
	pass_id: string;
	status: string;
	visitor_url: string;
	valid_from: string;
	valid_until: string;
}

export function InviteCompletion({ token }: { token: string }) {
	const [invite, setInvite] = useState<Invite | null>(null);
	const [error, setError] = useState("");
	const [activated, setActivated] = useState<ActivatedPass | null>(null);
	const [loading, setLoading] = useState(true);
	const [submitting, setSubmitting] = useState(false);

	useEffect(() => {
		let cancelled = false;
		fetch(`/api/fill/${encodeURIComponent(token)}`, { cache: "no-store" })
			.then(async (response) => {
				const body = (await response.json()) as Invite & ApiError;
				if (!response.ok) {
					throw new Error(body.error?.message ?? "This invitation is no longer available.");
				}
				if (!cancelled) {
					setInvite(body);
				}
			})
			.catch((cause: unknown) => {
				if (!cancelled) {
					setError(cause instanceof Error ? cause.message : "Could not load invitation.");
				}
			})
			.finally(() => {
				if (!cancelled) {
					setLoading(false);
				}
			});
		return () => {
			cancelled = true;
		};
	}, [token]);

	async function submit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setSubmitting(true);
		const formData = new FormData(event.currentTarget);
		try {
			const photo = formData.get("photo");
			if (photo instanceof File && photo.size > 0) {
				if (photo.size > 2 * 1024 * 1024) {
					setError("Photo must be 2 MB or smaller.");
					return;
				}
				const photoData = new FormData();
				photoData.set("photo", photo);
				const photoResponse = await fetch(`/api/fill/${encodeURIComponent(token)}/photo`, {
					method: "POST",
					body: photoData,
				});
				if (!photoResponse.ok) {
					const photoBody = (await photoResponse.json()) as ApiError;
					setError(photoBody.error?.message ?? "Could not upload photo.");
					return;
				}
			}
			const response = await fetch(`/api/fill/${encodeURIComponent(token)}`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					name: formData.get("name"),
					phone: formData.get("phone") || null,
					id_note: formData.get("idNote") || null,
					consent: formData.get("consent") === "on",
				}),
			});
			const body = (await response.json()) as ActivatedPass & ApiError;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not complete the invitation.");
				return;
			}
			setActivated(body);
		} catch {
			setError("The visitor service could not be reached.");
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<main className="mx-auto min-h-screen max-w-xl px-4 py-10 sm:py-16">
			<div className="rounded-3xl border border-border bg-card p-6 shadow-sm sm:p-8">
				<div className="mb-6 size-11 rounded-2xl bg-[#eaf0e9]" />
				<h1 className="text-2xl font-semibold tracking-tight">Complete your visitor pass</h1>
				{loading ? (
					<p className="mt-4 text-sm text-muted-foreground">Loading invitation…</p>
				) : error && !invite ? (
					<p className="mt-4 text-sm text-destructive" role="alert">{error}</p>
				) : activated ? (
					<div className="mt-5 space-y-3">
						<p className="rounded-xl bg-[#f0f4ef] p-4 text-sm">
							Your pass is ready. Show your visitor pass page at the gate.
						</p>
						<p className="text-sm text-muted-foreground">
							Valid until {new Date(activated.valid_until).toLocaleString()}.
						</p>
						<a className="inline-flex font-medium underline" href={activated.visitor_url}>
							View visitor pass
						</a>
					</div>
				) : invite ? (
					<>
						<p className="mt-2 text-sm text-muted-foreground">
							Invited by {invite.host_name} for {invite.purpose}.
						</p>
						<form className="mt-6 space-y-4" onSubmit={submit}>
							<label className="block space-y-2 text-sm font-medium">
								Your name
								<Input name="name" defaultValue={invite.visitor.name} required maxLength={200} />
							</label>
							<label className="block space-y-2 text-sm font-medium">
								Phone (optional)
								<Input name="phone" type="tel" defaultValue={invite.visitor.phone ?? ""} maxLength={50} />
							</label>
							<label className="block space-y-2 text-sm font-medium">
								Identity note (optional)
								<Input name="idNote" maxLength={500} />
							</label>
							<label className="block space-y-2 text-sm font-medium sm:col-span-2">
								Photo (optional, JPEG up to 2 MB)
								<Input name="photo" type="file" accept="image/jpeg" capture="user" />
							</label>
							<label className="flex items-start gap-3 text-sm text-muted-foreground">
								<input className="mt-1 accent-[#587662]" type="checkbox" name="consent" required />
								<span>I consent to sharing these details with the host and gate staff for this visit.</span>
							</label>
							{error && <p className="text-sm text-destructive" role="alert">{error}</p>}
							<Button className="w-full" type="submit" variant="primary" loading={submitting}>
								Confirm and activate pass
							</Button>
						</form>
					</>
				) : null}
			</div>
		</main>
	);
}
