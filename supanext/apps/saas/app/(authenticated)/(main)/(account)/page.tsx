import { getSession } from "@auth/lib/server";
import { Card } from "@repo/ui";
import {
	Activity,
	ArrowUpRight,
	BadgeCheck,
	Clock3,
	type LucideIcon,
	ShieldAlert,
	ShieldCheck,
	UsersRound,
} from "lucide-react";
import { redirect } from "next/navigation";

const BACKEND_URL = (
	process.env.BACKEND_URL ?? "http://localhost:8000"
).replace(/\/$/, "");

interface ApiResult<T> {
	data: T | null;
	error: string | null;
}

interface QueryResponse {
	answer: string;
	columns: string[];
	rows: unknown[][];
}

interface ScanFlag {
	gate_id?: string;
	scan_time?: string;
	status?: string;
	anomaly_score?: number;
	reasons?: string[];
}

interface FlagsResponse {
	model: string;
	flags: ScanFlag[];
	count: number;
}

interface HealthResponse {
	status: string;
}

async function requestApi<T>(
	path: string,
	init?: RequestInit,
): Promise<ApiResult<T>> {
	try {
		const response = await fetch(`${BACKEND_URL}${path}`, {
			...init,
			cache: "no-store",
		});

		if (!response.ok) {
			return {
				data: null,
				error: `The visitor service returned HTTP ${response.status}.`,
			};
		}

		return { data: (await response.json()) as T, error: null };
	} catch {
		return {
			data: null,
			error: "The visitor service could not be reached.",
		};
	}
}

function queryApi(question: string, limit = 1) {
	return requestApi<QueryResponse>("/ai/query", {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ question, limit }),
	});
}

function getCell(row: unknown[], columns: string[], column: string): string {
	const index = columns.indexOf(column);

	if (index < 0) {
		return "—";
	}

	const value = row[index];
	return typeof value === "string" || typeof value === "number"
		? String(value)
		: "—";
}

function formatScanTime(value: string): string {
	return value.replace("T", " · ").slice(0, 18);
}

export default async function AppStartPage() {
	const session = await getSession();

	if (!session) {
		redirect("/login");
	}

	const [
		health,
		visitors,
		activePasses,
		approvedToday,
		deniedToday,
		recentScans,
		flags,
	] = await Promise.all([
		requestApi<HealthResponse>("/health"),
		queryApi("how many visitors?"),
		queryApi("how many active passes?"),
		queryApi("how many approved scans today?"),
		queryApi("how many denied entries today?"),
		queryApi("show last 8 scans", 8),
		requestApi<FlagsResponse>(
			"/ai/anomaly/flags?limit=5&only_suspicious=true",
		),
	]);

	const firstName = session.user.name.trim().split(/\s+/)[0] || "there";
	const dateLabel = new Intl.DateTimeFormat("en", {
		weekday: "long",
		month: "long",
		day: "numeric",
	}).format(new Date());
	const errors = [
		visitors.error,
		activePasses.error,
		approvedToday.error,
		deniedToday.error,
		recentScans.error,
		flags.error,
	].filter((error): error is string => error !== null);
	const scans = recentScans.data;
	const suspiciousFlags = flags.data?.flags ?? [];
	const isOnline = health.data?.status === "ok";

	return (
		<div className="mx-auto max-w-7xl space-y-8 pb-8">
			<header className="flex flex-col justify-between gap-5 sm:flex-row sm:items-end">
				<div>
					<p className="mb-2 text-sm font-medium text-muted-foreground">
						{dateLabel}
					</p>
					<h1 className="text-3xl font-semibold tracking-tight text-foreground">
						Welcome back, {firstName}
					</h1>
					<p className="mt-2 text-sm text-muted-foreground">
						Here’s a calm, up-to-date view of visitor activity.
					</p>
				</div>
				<div
					className="inline-flex w-fit items-center gap-2 rounded-full border border-border bg-card px-3 py-2 text-sm text-muted-foreground"
					role="status"
				>
					<span
						className={`size-2 rounded-full ${isOnline ? "bg-emerald-500" : "bg-amber-500"}`}
					/>
					{isOnline
						? "Visitor service connected"
						: "Visitor service offline"}
				</div>
			</header>

			{errors.length > 0 && (
				<div
					className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900"
					role="status"
				>
					Some dashboard data is unavailable. Check that the backend
					is running at{" "}
					<code className="font-mono">{BACKEND_URL}</code>.
				</div>
			)}

			<section
				className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
				aria-label="Visitor statistics"
			>
				<MetricCard
					label="Registered visitors"
					value={visitors.data?.answer ?? "—"}
					icon={UsersRound}
					tone="sage"
				/>
				<MetricCard
					label="Active passes"
					value={activePasses.data?.answer ?? "—"}
					icon={BadgeCheck}
					tone="blue"
				/>
				<MetricCard
					label="Approved today"
					value={approvedToday.data?.answer ?? "—"}
					icon={ShieldCheck}
					tone="green"
				/>
				<MetricCard
					label="Denied today"
					value={deniedToday.data?.answer ?? "—"}
					icon={ShieldAlert}
					tone="amber"
				/>
			</section>

			<section className="grid gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(18rem,0.85fr)]">
				<Card className="overflow-hidden border-border bg-card p-0 shadow-sm">
					<div className="flex items-center justify-between border-b border-border px-5 py-4">
						<div>
							<h2 className="font-semibold text-foreground">
								Recent gate activity
							</h2>
							<p className="mt-1 text-sm text-muted-foreground">
								The latest visitor pass scans
							</p>
						</div>
						<Activity
							className="size-5 text-muted-foreground"
							aria-hidden="true"
						/>
					</div>
					{scans?.rows.length ? (
						<div className="overflow-x-auto">
							<table className="w-full text-left text-sm">
								<thead className="bg-muted/40 text-xs uppercase tracking-wide text-muted-foreground">
									<tr>
										<th className="px-5 py-3 font-medium">
											Gate
										</th>
										<th className="px-5 py-3 font-medium">
											Time
										</th>
										<th className="px-5 py-3 font-medium">
											Result
										</th>
									</tr>
								</thead>
								<tbody className="divide-y divide-border">
									{scans.rows.map((row, index) => {
										const status = getCell(
											row,
											scans.columns,
											"status",
										);

										return (
											<tr
												key={`${getCell(row, scans.columns, "id")}-${index}`}
											>
												<td className="px-5 py-4 font-medium text-foreground">
													{getCell(
														row,
														scans.columns,
														"gate_id",
													)}
												</td>
												<td className="whitespace-nowrap px-5 py-4 text-muted-foreground">
													{formatScanTime(
														getCell(
															row,
															scans.columns,
															"scan_time",
														),
													)}
												</td>
												<td className="px-5 py-4">
													<span
														className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${
															status ===
															"approved"
																? "bg-emerald-50 text-emerald-800"
																: "bg-rose-50 text-rose-800"
														}`}
													>
														{status}
													</span>
												</td>
											</tr>
										);
									})}
								</tbody>
							</table>
						</div>
					) : recentScans.error ? (
						<div className="px-5 py-12 text-center text-sm text-muted-foreground">
							Recent gate activity is unavailable.
						</div>
					) : (
						<div className="px-5 py-12 text-center">
							<Clock3
								className="mx-auto size-8 text-muted-foreground/60"
								aria-hidden="true"
							/>
							<p className="mt-3 font-medium text-foreground">
								No scans yet
							</p>
							<p className="mt-1 text-sm text-muted-foreground">
								Recent gate activity will appear here.
							</p>
						</div>
					)}
				</Card>

				<Card className="border-border bg-card p-5 shadow-sm">
					<div className="flex items-start justify-between gap-3">
						<div>
							<h2 className="font-semibold text-foreground">
								Security watch
							</h2>
							<p className="mt-1 text-sm text-muted-foreground">
								Recent scans flagged for review
							</p>
						</div>
						<ShieldAlert
							className="size-5 shrink-0 text-amber-700"
							aria-hidden="true"
						/>
					</div>
					{flags.error ? (
						<div className="mt-5 rounded-xl bg-amber-50/70 px-4 py-5 text-sm text-amber-950">
							Security watch is temporarily unavailable.
						</div>
					) : flags.data && suspiciousFlags.length > 0 ? (
						<ul className="mt-5 space-y-3">
							{suspiciousFlags.map((flag, index) => (
								<li
									key={`${flag.gate_id ?? "gate"}-${flag.scan_time ?? index}`}
									className="rounded-xl border border-amber-100 bg-amber-50/70 p-3"
								>
									<div className="flex items-center justify-between gap-3">
										<span className="font-medium text-foreground">
											{flag.gate_id ?? "Unknown gate"}
										</span>
										<span className="text-xs text-muted-foreground">
											{flag.status ?? "—"}
										</span>
									</div>
									<p className="mt-1 text-xs text-muted-foreground">
										{flag.scan_time
											? formatScanTime(flag.scan_time)
											: "Time unavailable"}
									</p>
									{flag.reasons &&
										flag.reasons.length > 0 && (
											<p className="mt-2 text-sm text-amber-950">
												{flag.reasons.join(" · ")}
											</p>
										)}
								</li>
							))}
						</ul>
					) : flags.data ? (
						<div className="mt-5 rounded-xl bg-emerald-50/70 px-4 py-5 text-sm text-emerald-900">
							No recent suspicious scans. Keep an eye on new
							activity here.
						</div>
					) : (
						<div className="mt-5 rounded-xl bg-muted px-4 py-5 text-sm text-muted-foreground">
							Security watch data is unavailable.
						</div>
					)}
					{flags.data?.model && (
						<p className="mt-4 text-xs text-muted-foreground">
							Detection: {flags.data.model.replaceAll("_", " ")}
						</p>
					)}
				</Card>
			</section>

			<Card className="border-border bg-[#f0f4ef] p-5 shadow-none sm:p-6">
				<div className="flex flex-col justify-between gap-5 md:flex-row md:items-center">
					<div>
						<p className="text-sm font-semibold text-foreground">
							A simple visitor flow
						</p>
						<p className="mt-1 text-sm text-muted-foreground">
							Issue a time-limited pass, scan it at the gate, and
							review the audit trail here.
						</p>
					</div>
					<div className="flex flex-wrap gap-2 text-sm">
						<FlowStep number="01" label="Issue a pass" />
						<ArrowUpRight
							className="my-auto size-4 text-muted-foreground"
							aria-hidden="true"
						/>
						<FlowStep number="02" label="Scan at the gate" />
						<ArrowUpRight
							className="my-auto size-4 text-muted-foreground"
							aria-hidden="true"
						/>
						<FlowStep number="03" label="Review activity" />
					</div>
				</div>
			</Card>
		</div>
	);
}

function MetricCard({
	label,
	value,
	icon: Icon,
	tone,
}: {
	label: string;
	value: string;
	icon: LucideIcon;
	tone: "sage" | "blue" | "green" | "amber";
}) {
	const iconTone = {
		sage: "bg-[#eaf0e9] text-[#587662]",
		blue: "bg-[#eaf0f3] text-[#5b7885]",
		green: "bg-emerald-50 text-emerald-800",
		amber: "bg-amber-50 text-amber-800",
	}[tone];

	return (
		<Card className="border-border bg-card p-5 shadow-sm">
			<div className="flex items-start justify-between gap-3">
				<div>
					<p className="text-sm text-muted-foreground">{label}</p>
					<p className="mt-3 text-2xl font-semibold tracking-tight text-foreground">
						{value}
					</p>
				</div>
				<div className={`rounded-xl p-2.5 ${iconTone}`}>
					<Icon className="size-5" aria-hidden="true" />
				</div>
			</div>
		</Card>
	);
}

function FlowStep({ number, label }: { number: string; label: string }) {
	return (
		<span className="inline-flex items-center gap-2 rounded-full border border-border bg-card px-3 py-2 text-foreground">
			<span className="text-xs font-semibold text-muted-foreground">
				{number}
			</span>
			{label}
		</span>
	);
}
