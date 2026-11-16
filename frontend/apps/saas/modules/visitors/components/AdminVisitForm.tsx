"use client";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { useEffect, useState, type FormEvent } from "react";

interface Host {
	id: string;
	name: string;
	username: string;
}

interface ApiError {
	error?: { message?: string };
}

interface VisitResponse {
	invite_url: string;
}

export function AdminVisitForm() {
	const [hosts, setHosts] = useState<Host[]>([]);
	const [error, setError] = useState("");
	const [result, setResult] = useState<VisitResponse | null>(null);
	const [submitting, setSubmitting] = useState(false);

	useEffect(() => {
		fetch("/api/hosts", { credentials: "same-origin", cache: "no-store" })
			.then(async (response) => {
				const body = (await response.json()) as { items: Host[] } & ApiError;
				if (!response.ok) {
					throw new Error(body.error?.message ?? "Could not load hosts.");
				}
				setHosts(body.items);
			})
			.catch((cause: unknown) => {
				setError(cause instanceof Error ? cause.message : "Could not load hosts.");
			});
	}, []);

	async function createInvite(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setResult(null);
		setSubmitting(true);
		const form = event.currentTarget;
		const data = new FormData(form);
		try {
			const response = await fetch("/api/visits", {
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					host_id: data.get("hostId"),
					visitor: {
						name: data.get("name"),
						phone: data.get("phone") || null,
					},
					purpose: data.get("purpose"),
					gate_id: data.get("gateId") || null,
					valid_from: new Date(String(data.get("validFrom"))).toISOString(),
					valid_until: new Date(String(data.get("validUntil"))).toISOString(),
					mode: data.get("mode"),
				}),
			});
			const body = (await response.json()) as VisitResponse & ApiError;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not create invite.");
				return;
			}
			setResult(body);
			form.reset();
		} catch {
			setError("The visitor service could not be reached.");
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<section id="admin-invite" className="rounded-2xl border border-border bg-card p-5 shadow-sm sm:p-6">
			<h2 className="font-semibold">Create an admin invitation</h2>
			<p className="mt-1 text-sm text-muted-foreground">
				Only administrators can issue hosted visitor invitations.
			</p>
			<form className="mt-5 grid gap-4 sm:grid-cols-2" onSubmit={createInvite}>
				<label className="space-y-2 text-sm font-medium">
					Host
					<select className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" name="hostId" required>
						<option value="">Select a host</option>
						{hosts.map((host) => (
							<option key={host.id} value={host.id}>{host.name} (@{host.username})</option>
						))}
					</select>
				</label>
				<label className="space-y-2 text-sm font-medium">
					Visitor name
					<Input name="name" required maxLength={200} />
				</label>
				<label className="space-y-2 text-sm font-medium">
					Phone (optional)
					<Input name="phone" type="tel" maxLength={50} />
				</label>
				<label className="space-y-2 text-sm font-medium">
					Purpose
					<Input name="purpose" required maxLength={300} />
				</label>
				<label className="space-y-2 text-sm font-medium">
					Valid from
					<Input name="validFrom" type="datetime-local" required />
				</label>
				<label className="space-y-2 text-sm font-medium">
					Valid until
					<Input name="validUntil" type="datetime-local" required />
				</label>
				<label className="space-y-2 text-sm font-medium">
					Pass type
					<select className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" name="mode" defaultValue="one_time">
						<option value="one_time">One-time entry</option>
						<option value="window">Multiple entry window</option>
					</select>
				</label>
				<div className="flex items-end">
					<Button className="w-full" type="submit" variant="primary" loading={submitting}>
						Create invite
					</Button>
				</div>
			</form>
			{error && <p className="mt-4 text-sm text-destructive" role="alert">{error}</p>}
			{result && (
				<div className="mt-5 rounded-xl border border-[#d7e3d8] bg-[#f0f4ef] p-4">
					<p className="font-medium">Invite created</p>
					<a className="mt-2 block break-all text-sm underline" href={result.invite_url}>{result.invite_url}</a>
				</div>
			)}
		</section>
	);
}
