"use client";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { useEffect, useState, type FormEvent } from "react";

interface HostOption {
	id: string;
	name: string;
	username: string;
}

interface GateOption {
	id: string;
	name: string;
}

interface MetadataResponse {
	hosts: HostOption[];
	gates: GateOption[];
}

interface ApiError {
	error?: { message?: string };
}

interface CreatedPass {
	visitor_url: string;
	valid_until: string;
	host_name: string;
}

export function SelfEntryForm() {
	const [metadata, setMetadata] = useState<MetadataResponse | null>(null);
	const [error, setError] = useState("");
	const [result, setResult] = useState<CreatedPass | null>(null);
	const [loading, setLoading] = useState(true);
	const [submitting, setSubmitting] = useState(false);

	useEffect(() => {
		fetch("/api/self-entry/meta", { cache: "no-store" })
			.then(async (response) => {
				const body = (await response.json()) as MetadataResponse & ApiError;
				if (!response.ok) {
					throw new Error(body.error?.message ?? "Could not load visitor options.");
				}
				setMetadata(body);
			})
			.catch((cause: unknown) => {
				setError(cause instanceof Error ? cause.message : "Could not load visitor options.");
			})
			.finally(() => setLoading(false));
	}, []);

	async function submit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setResult(null);
		setSubmitting(true);
		const form = event.currentTarget;
		const data = new FormData(form);
		try {
			const response = await fetch("/api/self-entry", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					host_id: data.get("hostId"),
					gate_id: data.get("gateId") || null,
					visitor: {
						name: data.get("name"),
						phone: data.get("phone") || null,
						id_note: data.get("idNote") || null,
					},
					purpose: data.get("purpose"),
					consent: data.get("consent") === "on",
				}),
			});
			const body = (await response.json()) as CreatedPass & ApiError;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not create your visitor pass.");
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
		<main className="mx-auto min-h-screen max-w-xl px-4 py-10 sm:py-16">
			<div className="rounded-3xl border border-border bg-card p-6 shadow-sm sm:p-8">
				<div className="mb-6 size-11 rounded-2xl bg-[#eaf0e9]" />
				<h1 className="text-2xl font-semibold tracking-tight">Get a visitor pass</h1>
				<p className="mt-2 text-sm leading-6 text-muted-foreground">
					Enter your details, choose the person you are visiting, and receive a QR code immediately.
				</p>
				{loading ? (
					<p className="mt-6 text-sm text-muted-foreground">Loading visitor options…</p>
				) : metadata ? (
					<form className="mt-6 space-y-4" onSubmit={submit}>
						<label className="block space-y-2 text-sm font-medium">
							Your name
							<Input name="name" required maxLength={200} />
						</label>
						<label className="block space-y-2 text-sm font-medium">
							Phone (optional)
							<Input name="phone" type="tel" maxLength={50} />
						</label>
						<label className="block space-y-2 text-sm font-medium">
							Person you are visiting
							<select className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" name="hostId" required>
								<option value="">Select a host</option>
								{metadata.hosts.map((host) => (
									<option key={host.id} value={host.id}>
										{host.name} (@{host.username})
									</option>
								))}
							</select>
						</label>
						<label className="block space-y-2 text-sm font-medium">
							Purpose of visit
							<Input name="purpose" required maxLength={300} />
						</label>
						<label className="block space-y-2 text-sm font-medium">
							Entry gate (optional)
							<select className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" name="gateId">
								<option value="">Any active gate</option>
								{metadata.gates.map((gate) => (
									<option key={gate.id} value={gate.id}>{gate.name}</option>
								))}
							</select>
						</label>
						<label className="block space-y-2 text-sm font-medium">
							Identity note (optional)
							<Input name="idNote" maxLength={500} />
						</label>
						<label className="flex items-start gap-3 text-sm text-muted-foreground">
							<input className="mt-1 accent-[#587662]" type="checkbox" name="consent" required />
							<span>I consent to sharing these details with the host and gate staff for this visit.</span>
						</label>
						{error && <p className="text-sm text-destructive" role="alert">{error}</p>}
						<Button className="w-full" type="submit" variant="primary" loading={submitting}>
							Create my QR pass
						</Button>
					</form>
				) : (
					<p className="mt-6 text-sm text-destructive" role="alert">{error}</p>
				)}
				{result && (
					<div className="mt-6 rounded-xl border border-[#d7e3d8] bg-[#f0f4ef] p-4">
						<p className="font-semibold">Your pass is ready</p>
						<p className="mt-1 text-sm text-muted-foreground">
							Show this QR pass at the gate. It is valid until {new Date(result.valid_until).toLocaleString()}.
						</p>
						<a className="mt-3 inline-block font-medium underline" href={result.visitor_url}>
							Open my visitor pass
						</a>
					</div>
				)}
			</div>
		</main>
	);
}
