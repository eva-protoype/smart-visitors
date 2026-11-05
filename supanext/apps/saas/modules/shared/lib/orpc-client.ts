import { createORPCClient, onError } from "@orpc/client";
import { RPCLink } from "@orpc/client/fetch";
import type { ApiRouterClient } from "@repo/api/orpc/router";

const link = new RPCLink({
	url: () => {
		if (typeof window === "undefined") {
			throw new Error("RPCLink is not allowed on the server side.");
		}
		return `${window.location.origin}/api/rpc`;
	},
	headers: async () => ({}),
	interceptors: [
		onError((error) => {
			if (error instanceof Error && error.name === "AbortError") {
				return;
			}

			console.error(error);
		}),
	],
});

export const orpcClient: ApiRouterClient = createORPCClient(link);
