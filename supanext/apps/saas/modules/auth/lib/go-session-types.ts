export interface GoUser {
	id: string;
	role: "admin" | "host" | "guard";
	username: string;
	name: string;
	email?: string | null;
	must_reset_password: boolean;
}

export interface GoSession {
	user: GoUser;
}
