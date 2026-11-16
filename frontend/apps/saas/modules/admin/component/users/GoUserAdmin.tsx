"use client";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { useCallback, useEffect, useState, type FormEvent } from "react";

interface ApiError {
	error?: { message?: string };
}

interface UserRecord {
	id: string;
	role: "admin" | "host" | "guard";
	username: string;
	name: string;
	email: string | null;
	disabled_at: string | null;
	must_reset_password: boolean;
}

interface Credentials {
	temp_password: string;
	temp_password_expires_at: string;
	user?: UserRecord;
}

export function GoUserAdmin() {
	const [users, setUsers] = useState<UserRecord[]>([]);
	const [error, setError] = useState("");
	const [loading, setLoading] = useState(true);
	const [submitting, setSubmitting] = useState(false);
	const [credentials, setCredentials] = useState<Credentials | null>(null);

	const loadUsers = useCallback(async () => {
		setError("");
		try {
			const response = await fetch("/api/users?limit=100", {
				credentials: "same-origin",
				cache: "no-store",
			});
			const body = (await response.json()) as { items: UserRecord[] } & ApiError;
			if (!response.ok) {
				throw new Error(body.error?.message ?? "Could not load accounts.");
			}
			setUsers(body.items);
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : "Could not load accounts.");
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		void loadUsers();
	}, [loadUsers]);

	async function createUser(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		setError("");
		setCredentials(null);
		setSubmitting(true);
		const form = event.currentTarget;
		const data = new FormData(form);
		try {
			const response = await fetch("/api/users", {
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					name: data.get("name"),
					username: data.get("username"),
					role: data.get("role"),
					email: data.get("email") || null,
				}),
			});
			const body = (await response.json()) as Credentials & ApiError;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not create account.");
				return;
			}
			setCredentials(body);
			form.reset();
			await loadUsers();
		} catch {
			setError("The visitor service could not be reached.");
		} finally {
			setSubmitting(false);
		}
	}

	async function resetPassword(user: UserRecord) {
		setError("");
		setCredentials(null);
		try {
			const response = await fetch(`/api/users/${user.id}/resetpassword`, {
				method: "POST",
				credentials: "same-origin",
			});
			const body = (await response.json()) as Credentials & ApiError;
			if (!response.ok) {
				setError(body.error?.message ?? "Could not reset the password.");
				return;
			}
			setCredentials(body);
			await loadUsers();
		} catch {
			setError("The visitor service could not be reached.");
		}
	}

	async function disableUser(user: UserRecord) {
		if (!window.confirm(`Disable ${user.name}'s account?`)) {
			return;
		}
		setError("");
		try {
			const response = await fetch(`/api/users/${user.id}/disable`, {
				method: "POST",
				credentials: "same-origin",
			});
			if (!response.ok) {
				const body = (await response.json()) as ApiError;
				setError(body.error?.message ?? "Could not disable the account.");
				return;
			}
			await loadUsers();
		} catch {
			setError("The visitor service could not be reached.");
		}
	}

	return (
		<div className="space-y-6">
			<section className="rounded-2xl border border-border bg-card p-5 shadow-sm sm:p-6">
				<h2 className="font-semibold">Create a team account</h2>
				<p className="mt-1 text-sm text-muted-foreground">
					New accounts receive a temporary password and must change it at first sign-in.
				</p>
				<form className="mt-5 grid gap-4 sm:grid-cols-2" onSubmit={createUser}>
					<label className="space-y-2 text-sm font-medium">
						Full name
						<Input name="name" required maxLength={200} />
					</label>
					<label className="space-y-2 text-sm font-medium">
						Username
						<Input name="username" required maxLength={100} />
					</label>
					<label className="space-y-2 text-sm font-medium">
						Email (optional)
						<Input name="email" type="email" maxLength={320} />
					</label>
					<label className="space-y-2 text-sm font-medium">
						Role
						<select
							className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
							name="role"
							defaultValue="host"
						>
							<option value="host">Host</option>
							<option value="guard">Guard</option>
						</select>
					</label>
					<div className="sm:col-span-2">
						<Button type="submit" variant="primary" loading={submitting}>
							Create account
						</Button>
					</div>
				</form>
				{error && <p className="mt-4 text-sm text-destructive" role="alert">{error}</p>}
				{credentials && (
					<div className="mt-5 rounded-xl border border-[#d7e3d8] bg-[#f0f4ef] p-4">
						<p className="font-semibold">Show this temporary password once</p>
						<code className="mt-2 block select-all break-all font-mono text-sm">
							{credentials.temp_password}
						</code>
						<p className="mt-2 text-xs text-muted-foreground">
							Expires {new Date(credentials.temp_password_expires_at).toLocaleString()}.
						</p>
					</div>
				)}
			</section>

			<section className="overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
				<div className="border-b border-border px-5 py-4">
					<h2 className="font-semibold">Team accounts</h2>
				</div>
				{loading ? (
					<p className="p-5 text-sm text-muted-foreground">Loading accounts…</p>
				) : users.length === 0 ? (
					<p className="p-5 text-sm text-muted-foreground">No accounts found.</p>
				) : (
					<div className="overflow-x-auto">
						<table className="w-full text-left text-sm">
							<thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
								<tr>
									<th className="px-5 py-3 font-medium">Name</th>
									<th className="px-5 py-3 font-medium">Username</th>
									<th className="px-5 py-3 font-medium">Role</th>
									<th className="px-5 py-3 font-medium">Status</th>
									<th className="px-5 py-3 font-medium">Actions</th>
								</tr>
							</thead>
							<tbody className="divide-y divide-border">
								{users.map((user) => (
									<tr key={user.id}>
										<td className="px-5 py-4 font-medium">{user.name}</td>
										<td className="px-5 py-4 text-muted-foreground">{user.username}</td>
										<td className="px-5 py-4 capitalize">{user.role}</td>
										<td className="px-5 py-4 text-muted-foreground">
											{user.disabled_at ? "Disabled" : user.must_reset_password ? "Password reset required" : "Active"}
										</td>
										<td className="px-5 py-4">
											<div className="flex gap-2">
												<Button type="button" variant="secondary" size="sm" disabled={Boolean(user.disabled_at)} onClick={() => void resetPassword(user)}>
													Reset
												</Button>
												{!user.disabled_at && (
													<Button type="button" variant="ghost" size="sm" onClick={() => void disableUser(user)}>
														Disable
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
		</div>
	);
}
