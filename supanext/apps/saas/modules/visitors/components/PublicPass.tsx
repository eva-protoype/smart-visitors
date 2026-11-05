"use client";

import { useEffect, useState } from "react";
import QRCode from "react-qr-code";

interface ApiError {
	error?: { message?: string };
}

interface PassDetails {
	name: string;
	host_name: string;
	purpose: string;
	status: string;
	mode: string;
	valid_from: string;
	valid_until: string;
}

export function PublicPass({ token }: { token: string }) {
	const [pass, setPass] = useState<PassDetails | null>(null);
	const [error, setError] = useState("");

	useEffect(() => {
		let cancelled = false;
		fetch(`/api/p/${encodeURIComponent(token)}`, { cache: "no-store" })
			.then(async (response) => {
				const body = (await response.json()) as PassDetails & ApiError;
				if (!response.ok) {
					throw new Error(body.error?.message ?? "Pass not found.");
				}
				if (!cancelled) {
					setPass(body);
				}
			})
			.catch((cause: unknown) => {
				if (!cancelled) {
					setError(cause instanceof Error ? cause.message : "Could not load pass.");
				}
			});
		return () => {
			cancelled = true;
		};
	}, [token]);

	return (
		<main className="mx-auto min-h-screen max-w-xl px-4 py-10 sm:py-16">
			<section className="rounded-3xl border border-border bg-card p-6 shadow-sm sm:p-8">
				<div className="mb-6 size-11 rounded-2xl bg-[#eaf0e9]" />
				<h1 className="text-2xl font-semibold tracking-tight">Visitor pass</h1>
				{error ? (
					<p className="mt-4 text-sm text-destructive" role="alert">{error}</p>
				) : pass ? (
					<>
						<div className="mt-6 flex justify-center rounded-2xl bg-white p-6">
							<QRCode title="Visitor pass QR code" value={`sv1:${token}`} size={192} />
						</div>
						<dl className="mt-6 grid gap-4 text-sm sm:grid-cols-2">
							<Detail label="Visitor" value={pass.name} />
							<Detail label="Host" value={pass.host_name} />
							<Detail label="Purpose" value={pass.purpose} />
							<Detail label="Status" value={pass.status} />
							<Detail label="Valid from" value={new Date(pass.valid_from).toLocaleString()} />
							<Detail label="Valid until" value={new Date(pass.valid_until).toLocaleString()} />
							<p className="rounded-xl bg-[#f0f4ef] p-4 text-muted-foreground sm:col-span-2">
								Show this code to gate staff. Access is confirmed by the visitor service.
							</p>
						</dl>
					</>
				) : (
					<p className="mt-4 text-sm text-muted-foreground">Loading pass…</p>
				)}
			</section>
		</main>
	);
}

function Detail({ label, value }: { label: string; value: string }) {
	return (
		<div className="rounded-xl border border-border p-4">
			<dt className="text-muted-foreground">{label}</dt>
			<dd className="mt-1 font-medium capitalize">{value}</dd>
		</div>
	);
}
