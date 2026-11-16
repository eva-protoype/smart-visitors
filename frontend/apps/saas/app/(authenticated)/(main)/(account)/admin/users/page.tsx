import { GoUserAdmin } from "@admin/component/users/GoUserAdmin";

export default function AdminUsersPage() {
	return (
		<div className="space-y-6">
			<header>
				<h1 className="text-2xl font-semibold tracking-tight">Team accounts</h1>
				<p className="mt-1 text-sm text-muted-foreground">
					Manage host and guard access.
				</p>
			</header>
			<GoUserAdmin />
		</div>
	);
}
