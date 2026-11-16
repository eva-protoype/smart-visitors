"use client";

import { Button } from "@repo/ui/components/button";
import { Download, RefreshCw, UsersRound } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

interface VisitorRow {
	visitor_id: string;
	name: string;
	phone?: string;
	host_name: string;
	host_username: string;
	purpose: string;
	pass_status: string;
	gate_name?: string;
	created_at: string;
	valid_from: string;
	valid_until: string;
}

interface ApiError {
	error?: { message?: string };
}

function localDateTime(value: Date) {
	const offset = value.getTimezoneOffset() * 60000;
	return new Date(value.getTime() - offset).toISOString().slice(0, 16);
}

export function AdminVisitorDirectory() {
	const now = new Date();
	const [from, setFrom] = useState(localDateTime(new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)));
	const [to, setTo] = useState(localDateTime(now));
	const [rows, setRows] = useState<VisitorRow[]>([]);
	const [error, setError] = useState("");
	const [loading, setLoading] = useState(false);
	const [downloading, setDownloading] = useState(false);

	const query = useCallback(() => {
		const params = new URLSearchParams({
			from: new Date(from).toISOString(),
			to: new Date(to).toISOString(),
		});
		return params.toString();
	}, [from, to]);

	const load = useCallback(async () => {
		setError("");
		setLoading(true);
		try {
			const response = await fetch(`/api/admin/visitors?${query()}`, {
				credentials: "same-origin",
				cache: "no-store",
			});
			const body = (await response.json()) as { items: VisitorRow[] } & ApiError;
			if (!response.ok) {
				throw new Error(body.error?.message ?? "Could not load visitor records.");
			}
			setRows(body.items);
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : "Could not load visitor records.");
		} finally {
			setLoading(false);
		}
	}, [query]);

	useEffect(() => {
		void load();
	}, [load]);

	async function downloadCSV() {
		setError("");
		setDownloading(true);
		try {
			const response = await fetch(`/api/admin/visitors.csv?${query()}`, {
				credentials: "same-origin",
			});
			if (!response.ok) {
				const body = (await response.json()) as ApiError;
				throw new Error(body.error?.message ?? "Could not download visitor records.");
			}
			const blob = await response.blob();
			const url = URL.createObjectURL(blob);
			const link = document.createElement("a");
			link.href = url;
			link.download = "visitor-records.csv";
			link.click();
			URL.revokeObjectURL(url);
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : "Could not download visitor records.");
		} finally {
			setDownloading(false);
		}
	}

	return (
		<section id="visitors" className="overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
			<div className="flex flex-col gap-4 border-b border-border px-5 py-5 sm:px-6">
				<div className="flex items-start justify-between gap-3">
					<div>
						<h2 className="font-semibold">Visitors and hosts</h2>
						<p className="mt-1 text-sm text-muted-foreground">
							Review visitor details, assigned hosts, pass status, and entry windows.
						</p>
					</div>
					<UsersRound className="size-5 text-[#587662]" aria-hidden="true" />
				</div>
				<div className="grid gap-3 sm:grid-cols-[1fr_1fr_auto_auto] sm:items-end">
					<label className="space-y-2 text-sm font-medium">
						From
						<input className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" type="datetime-local" value={from} onChange={(event) => setFrom(event.target.value)} />
					</label>
					<label className="space-y-2 text-sm font-medium">
						To
						<input className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm" type="datetime-local" value={to} onChange={(event) => setTo(event.target.value)} />
					</label>
					<Button type="button" variant="secondary" onClick={() => void load()} loading={loading}>
						<RefreshCw className="size-4" aria-hidden="true" /> Refresh
					</Button>
					<Button type="button" variant="primary" onClick={() => void downloadCSV()} loading={downloading}>
						<Download className="size-4" aria-hidden="true" /> Download CSV
					</Button>
				</div>
			</div>
			{error && <p className="px-5 pt-4 text-sm text-destructive sm:px-6" role="alert">{error}</p>}
			{loading && rows.length === 0 ? (
				<p className="p-6 text-sm text-muted-foreground">Loading visitor records…</p>
			) : rows.length === 0 ? (
				<p className="p-6 text-sm text-muted-foreground">No visitor records found for this time range.</p>
			) : (
				<div className="overflow-x-auto">
					<table className="w-full min-w-[900px] text-left text-sm">
						<thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
							<tr>
								<th className="px-5 py-3 font-medium">Visitor</th>
								<th className="px-5 py-3 font-medium">Host</th>
								<th className="px-5 py-3 font-medium">Purpose</th>
								<th className="px-5 py-3 font-medium">Status</th>
								<th className="px-5 py-3 font-medium">Created</th>
								<th className="px-5 py-3 font-medium">Valid until</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-border">
							{rows.map((row) => (
								<tr key={`${row.visitor_id}-${row.created_at}`}>
									<td className="px-5 py-4">
										<p className="font-medium">{row.name}</p>
										{row.phone && <p className="mt-1 text-xs text-muted-foreground">{row.phone}</p>}
									</td>
									<td className="px-5 py-4">
										<p className="font-medium">{row.host_name}</p>
										<p className="mt-1 text-xs text-muted-foreground">@{row.host_username}</p>
									</td>
									<td className="px-5 py-4 text-muted-foreground">{row.purpose}</td>
									<td className="px-5 py-4 capitalize">{row.pass_status}</td>
									<td className="whitespace-nowrap px-5 py-4 text-muted-foreground">{new Date(row.created_at).toLocaleString()}</td>
									<td className="whitespace-nowrap px-5 py-4 text-muted-foreground">{new Date(row.valid_until).toLocaleString()}</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			)}
		</section>
	);
}
