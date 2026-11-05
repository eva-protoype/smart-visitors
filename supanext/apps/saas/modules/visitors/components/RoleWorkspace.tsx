"use client";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import {
	useCallback,
	useEffect,
	useState,
	type FormEvent,
} from "react";

type Role = "host" | "guard";

interface ApiFailure {
	error?: {
		message?: string;
	};
}

interface Gate {
	id: string;
	name: string;
	active: boolean;
}

interface GateResponse {
	items: Gate[];
}

interface VisitResponse {
	pass_id: string;
	invite_url: string;
	status: string;
}

interface ScanResponse {
	approved: boolean;
	reason: string;
	message: string;
	name?: string;
	photo?: string;
	host_name?: string;
	purpose?: string;
}

interface Visit {
	id: string;
	visitor_id: string;
	status: string;
	mode: string;
	purpose: string;
	valid_from: string;
	valid_until: string;
	visitor_name: string;
	host_name: string;
}

interface WalkInResponse {
	pass_id: string;
	status: string;
	token: string;
}

export function RoleWorkspace({ role }: { role: Role }) {
	return role === "host" ? <HostWorkspace /> : <GuardWorkspace />;
}

function HostWorkspace() {
	const [error, setError] = useState("");
	const [result, setResult] = useState<VisitResponse | null>(null);
	const [submitting, setSubmitting] = useState(false);

	async function createVisit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setResult(null);
		setSubmitting(true);

		const form = event.currentTarget;
		const formData = new FormData(form);
		try {
			const response = await fetch("/api/visits", {
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					visitor: {
						name: formData.get("name"),
						phone: formData.get("phone") || null,
						id_note: formData.get("idNote") || null,
					},
					purpose: formData.get("purpose"),
					valid_from: new Date(String(formData.get("validFrom"))).toISOString(),
					valid_until: new Date(String(formData.get("validUntil"))).toISOString(),
					mode: formData.get("mode"),
				}),
			});
			const body = (await response.json()) as VisitResponse & ApiFailure;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not create the visit.");
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
		<div className="grid gap-5 lg:grid-cols-[minmax(0,1.2fr)_minmax(18rem,0.8fr)]">
			<section className="rounded-2xl border border-border bg-card p-5 shadow-sm sm:p-6">
				<h2 className="font-semibold">Invite a visitor</h2>
				<p className="mt-1 text-sm text-muted-foreground">
					Create a time-limited invite for your guest to complete.
				</p>
				<form className="mt-5 grid gap-4 sm:grid-cols-2" onSubmit={createVisit}>
					<label className="space-y-2 text-sm font-medium">
						Visitor name
						<Input name="name" required maxLength={200} />
					</label>
					<label className="space-y-2 text-sm font-medium">
						Phone (optional)
						<Input name="phone" type="tel" maxLength={50} />
					</label>
					<label className="space-y-2 text-sm font-medium sm:col-span-2">
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
						<select
							className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
							name="mode"
							defaultValue="one_time"
						>
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
						<p className="mt-1 break-all text-sm text-muted-foreground">
							{result.invite_url}
						</p>
						<a className="mt-3 inline-block text-sm font-medium underline" href={result.invite_url}>
							Open visitor invite
						</a>
					</div>
				)}
			</section>
			<aside className="rounded-2xl border border-border bg-[#f0f4ef] p-5 sm:p-6">
				<h2 className="font-semibold">A smooth arrival</h2>
				<p className="mt-2 text-sm leading-6 text-muted-foreground">
					Your visitor completes the invite and presents their QR pass at the gate.
					Use the visit list in the dashboard to review upcoming and active visits.
				</p>
			</aside>
			<div className="lg:col-span-2">
				<HostVisits />
			</div>
		</div>
	);
}

function HostVisits() {
	const [scope, setScope] = useState("upcoming");
	const [visits, setVisits] = useState<Visit[]>([]);
	const [error, setError] = useState("");
	const [loading, setLoading] = useState(true);
	const [details, setDetails] = useState<Record<string, unknown> | null>(null);
	const [photoVisitorID, setPhotoVisitorID] = useState<string | null>(null);

	const loadVisits = useCallback(async () => {
		setLoading(true);
		setError("");
		try {
			const response = await fetch(
				`/api/visits?scope=${scope}&limit=100`,
				{ credentials: "same-origin", cache: "no-store" },
			);
			const body = (await response.json()) as {
				items: Visit[];
				error?: { message?: string };
			};
			if (!response.ok) {
				throw new Error(body.error?.message ?? "Could not load visits.");
			}
			setVisits(body.items);
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : "Could not load visits.");
		} finally {
			setLoading(false);
		}
	}, [scope]);

	useEffect(() => {
		void loadVisits();
	}, [loadVisits]);

	async function decrypt(visit: Visit) {
		setError("");
		setDetails(null);
		setPhotoVisitorID(null);
		try {
			const response = await fetch(`/api/passes/${visit.id}/details`, {
				credentials: "same-origin",
				cache: "no-store",
			});
			const body = (await response.json()) as Record<string, unknown> & ApiFailure;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not decrypt pass details.");
				return;
			}
			setDetails(body);
			setPhotoVisitorID(visit.visitor_id);
		} catch {
			setError("The visitor service could not be reached.");
		}
	}

	async function revoke(passId: string) {
		const reason = window.prompt("Why are you revoking this pass?");
		if (!reason?.trim()) {
			return;
		}
		setError("");
		try {
			const response = await fetch(`/api/passes/${passId}/revoke`, {
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ reason: reason.trim() }),
			});
			if (!response.ok) {
				const body = (await response.json()) as ApiFailure;
				setError(body.error?.message ?? "Could not revoke pass.");
				return;
			}
			await loadVisits();
		} catch {
			setError("The visitor service could not be reached.");
		}
	}

	return (
		<section className="overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
			<div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
				<div>
					<h2 className="font-semibold">Your visits</h2>
					<p className="mt-1 text-sm text-muted-foreground">
						Review visit details or revoke a pass.
					</p>
				</div>
				<select
					className="h-10 rounded-md border border-input bg-background px-3 text-sm"
					value={scope}
					onChange={(event) => setScope(event.target.value)}
					aria-label="Visit status"
				>
					<option value="upcoming">Upcoming</option>
					<option value="active">Active</option>
					<option value="ended">Ended</option>
				</select>
			</div>
			{error && <p className="px-5 pt-4 text-sm text-destructive" role="alert">{error}</p>}
			{details && (
				<div className="m-5 rounded-xl border border-[#d7e3d8] bg-[#f0f4ef] p-4">
					<div className="flex items-start justify-between gap-3">
						<h3 className="font-medium">Decrypted visitor details</h3>
						<button className="text-sm underline" onClick={() => setDetails(null)} type="button">
							Close
						</button>
					</div>
					{photoVisitorID && (
						<img
							alt="Visitor"
							className="mt-4 size-24 rounded-xl object-cover"
							src={`/api/visitors/${encodeURIComponent(photoVisitorID)}/photo`}
						/>
					)}
					<pre className="mt-3 overflow-x-auto whitespace-pre-wrap text-xs">
						{JSON.stringify(details, null, 2)}
					</pre>
				</div>
			)}
			{loading ? (
				<p className="p-5 text-sm text-muted-foreground">Loading visits…</p>
			) : visits.length === 0 ? (
				<p className="p-5 text-sm text-muted-foreground">No visits in this view.</p>
			) : (
				<div className="overflow-x-auto">
					<table className="w-full text-left text-sm">
						<thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
							<tr>
								<th className="px-5 py-3 font-medium">Visitor</th>
								<th className="px-5 py-3 font-medium">Purpose</th>
								<th className="px-5 py-3 font-medium">Validity</th>
								<th className="px-5 py-3 font-medium">Status</th>
								<th className="px-5 py-3 font-medium">Actions</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-border">
							{visits.map((visit) => (
								<tr key={visit.id}>
									<td className="px-5 py-4 font-medium">{visit.visitor_name}</td>
									<td className="px-5 py-4 text-muted-foreground">{visit.purpose}</td>
									<td className="whitespace-nowrap px-5 py-4 text-muted-foreground">
										{new Date(visit.valid_from).toLocaleString()} – {new Date(visit.valid_until).toLocaleString()}
									</td>
									<td className="px-5 py-4 capitalize">{visit.status}</td>
									<td className="px-5 py-4">
										<div className="flex gap-2">
											<Button type="button" size="sm" variant="secondary" onClick={() => void decrypt(visit)}>
												Details
											</Button>
											{visit.status === "active" && (
												<Button type="button" size="sm" variant="ghost" onClick={() => void revoke(visit.id)}>
													Revoke
												</Button>
											)}
										</div>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</section>
	);
}

function GuardWorkspace() {
	const [gates, setGates] = useState<Gate[]>([]);
	const [error, setError] = useState("");
	const [result, setResult] = useState<ScanResponse | null>(null);
	const [loadingGates, setLoadingGates] = useState(true);
	const [scanning, setScanning] = useState(false);

	useEffect(() => {
		let cancelled = false;
		fetch("/api/gates", { credentials: "same-origin", cache: "no-store" })
			.then(async (response) => {
				const body = (await response.json()) as GateResponse & ApiFailure;
				if (!response.ok) {
					throw new Error(body.error?.message ?? "Could not load gates.");
				}
				if (!cancelled) {
					setGates(body.items.filter((gate) => gate.active));
				}
			})
			.catch((cause: unknown) => {
				if (!cancelled) {
					setError(cause instanceof Error ? cause.message : "Could not load gates.");
				}
			})
			.finally(() => {
				if (!cancelled) {
					setLoadingGates(false);
				}
			});
		return () => {
			cancelled = true;
		};
	}, []);

	async function scan(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setResult(null);
		setScanning(true);
		const form = event.currentTarget;
		const formData = new FormData(form);
		try {
			const response = await fetch("/api/scan", {
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					token: formData.get("token"),
					gate_id: formData.get("gateId"),
					direction: formData.get("direction"),
				}),
			});
			const body = (await response.json()) as ScanResponse & ApiFailure;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not process scan.");
				return;
			}
			setResult(body);
			form.reset();
		} catch {
			setError("The visitor service could not be reached.");
		} finally {
			setScanning(false);
		}
	}

	return (
		<div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(18rem,0.7fr)]">
			<section className="rounded-2xl border border-border bg-card p-5 shadow-sm sm:p-6">
				<h2 className="font-semibold">Gate scan</h2>
				<p className="mt-1 text-sm text-muted-foreground">
					Enter or scan a visitor code to verify access.
				</p>
				<form className="mt-5 space-y-4" onSubmit={scan}>
					<label className="block space-y-2 text-sm font-medium">
						Pass code
						<Input
							name="token"
							autoComplete="off"
							placeholder="sv1:…"
							required
						/>
					</label>
					<div className="grid gap-4 sm:grid-cols-2">
						<label className="space-y-2 text-sm font-medium">
							Gate
							<select
								className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
								name="gateId"
								required
								disabled={loadingGates || gates.length === 0}
							>
								<option value="">Select a gate</option>
								{gates.map((gate) => (
									<option key={gate.id} value={gate.id}>
										{gate.name}
									</option>
								))}
							</select>
						</label>
						<label className="space-y-2 text-sm font-medium">
							Direction
							<select
								className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
								name="direction"
								defaultValue="entry"
							>
								<option value="entry">Entry</option>
								<option value="exit">Exit</option>
							</select>
						</label>
					</div>
					<Button
						className="w-full"
						type="submit"
						variant="primary"
						loading={scanning}
						disabled={loadingGates || gates.length === 0}
					>
						Check pass
					</Button>
				</form>
				{error && <p className="mt-4 text-sm text-destructive" role="alert">{error}</p>}
				{result && (
					<div
						className={`mt-5 rounded-xl border p-4 ${
							result.approved
								? "border-emerald-200 bg-emerald-50 text-emerald-950"
								: "border-amber-200 bg-amber-50 text-amber-950"
						}`}
						role="status"
					>
						<p className="font-semibold">{result.approved ? "Access approved" : "Access denied"}</p>
						<p className="mt-1 text-sm">{result.message || result.reason}</p>
						{result.name && <p className="mt-2 text-sm">{result.name} · {result.host_name}</p>}
						{result.photo && (
							<img
								alt={`Visitor ${result.name ?? ""}`}
								className="mt-3 size-20 rounded-xl object-cover"
								src={`data:image/jpeg;base64,${result.photo}`}
							/>
						)}
					</div>
				)}
			</section>
			<aside className="rounded-2xl border border-border bg-[#f0f4ef] p-5 sm:p-6">
				<h2 className="font-semibold">Check carefully</h2>
				<p className="mt-2 text-sm leading-6 text-muted-foreground">
					Only approve passes when the server confirms the pass, gate,
					direction, and validity window. If the service is offline, fail closed.
				</p>
			</aside>
			<div className="lg:col-span-2">
				<WalkInForm />
			</div>
		</div>
	);
}

function WalkInForm() {
	const [error, setError] = useState("");
	const [result, setResult] = useState<WalkInResponse | null>(null);
	const [submitting, setSubmitting] = useState(false);

	async function createWalkIn(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setResult(null);
		setSubmitting(true);
		const form = event.currentTarget;
		const data = new FormData(form);
		const validFrom = String(data.get("validFrom") ?? "");
		const validUntil = String(data.get("validUntil") ?? "");
		const stickerToken = String(data.get("stickerToken") ?? "").trim();
		try {
			const response = await fetch(
				stickerToken ? "/api/stickers/claim" : "/api/walkins",
				{
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					...(stickerToken ? { sticker_token: stickerToken } : {}),
					host_username: data.get("hostUsername"),
					visitor: {
						name: data.get("name"),
						phone: data.get("phone") || null,
						id_note: data.get("idNote") || null,
					},
					purpose: data.get("purpose"),
					consent_note: data.get("consentNote"),
					...(validFrom
						? { valid_from: new Date(validFrom).toISOString() }
						: {}),
					...(validUntil
						? { valid_until: new Date(validUntil).toISOString() }
						: {}),
				}),
				},
			);
			const body = (await response.json()) as WalkInResponse & ApiFailure;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not create walk-in pass.");
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
		<section className="rounded-2xl border border-border bg-card p-5 shadow-sm sm:p-6">
			<h2 className="font-semibold">Walk-in visitor</h2>
			<p className="mt-1 text-sm text-muted-foreground">
				Issue a pass at the gate and record visitor consent.
			</p>
			<form className="mt-5 grid gap-4 sm:grid-cols-2" onSubmit={createWalkIn}>
				<label className="space-y-2 text-sm font-medium">
					Host username
					<Input name="hostUsername" required maxLength={100} />
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
					Identity note (optional)
					<Input name="idNote" maxLength={500} />
				</label>
				<label className="space-y-2 text-sm font-medium sm:col-span-2">
					Purpose
					<Input name="purpose" required maxLength={300} />
				</label>
				<label className="space-y-2 text-sm font-medium">
					Valid from (optional)
					<Input name="validFrom" type="datetime-local" />
				</label>
				<label className="space-y-2 text-sm font-medium">
					Valid until (optional)
					<Input name="validUntil" type="datetime-local" />
				</label>
				<label className="space-y-2 text-sm font-medium sm:col-span-2">
					Free sticker token (optional)
					<Input name="stickerToken" autoComplete="off" />
				</label>
				<label className="space-y-2 text-sm font-medium sm:col-span-2">
					Consent note
					<Input name="consentNote" required maxLength={500} />
				</label>
				<div className="sm:col-span-2">
					<Button type="submit" variant="primary" loading={submitting}>
						Create walk-in pass
					</Button>
				</div>
			</form>
			{error && <p className="mt-4 text-sm text-destructive" role="alert">{error}</p>}
			{result && (
				<div className="mt-5 rounded-xl border border-[#d7e3d8] bg-[#f0f4ef] p-4">
					<p className="font-medium">Walk-in pass created</p>
					<p className="mt-2 text-sm text-muted-foreground">
						Show this pass code at the gate:
					</p>
					<code className="mt-2 block select-all break-all font-mono text-sm">
						{result.token}
					</code>
				</div>
			)}
		</section>
	);
}
