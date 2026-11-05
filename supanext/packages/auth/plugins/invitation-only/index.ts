import { getPendingInvitationByEmail } from "@repo/database";
import type { BetterAuthPlugin } from "better-auth";
import { APIError, createAuthMiddleware } from "better-auth/api";
import { config } from "../../config";

export const invitationOnlyPlugin = () =>
	({
		id: "invitationOnlyPlugin",
		hooks: {
			before: [
				{
					matcher: (context) =>
						!!context.path?.startsWith("/sign-up/email"),
					handler: createAuthMiddleware(async (ctx) => {
						if (config.enableSignup) {
							return;
						}

						const { email } = ctx.body;

						// check if there is an invitation for the email
						const hasInvitation =
							await getPendingInvitationByEmail(email);

						if (!hasInvitation) {
							throw new APIError("BAD_REQUEST", {
								code: "INVALID_INVITATION",
								message: "No invitation found for this email",
							});
						}
					}),
				},
			],
		},
		$ERROR_CODES: {
			INVALID_INVITATION: {
				code: "INVALID_INVITATION",
				message: "No invitation found for this email",
			},
		},
	}) satisfies BetterAuthPlugin;
