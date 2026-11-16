import { InviteCompletion } from "@visitors/components/InviteCompletion";

export default async function FillInvitePage({
	params,
}: {
	params: Promise<{ token: string }>;
}) {
	const { token } = await params;
	return <InviteCompletion token={token} />;
}
