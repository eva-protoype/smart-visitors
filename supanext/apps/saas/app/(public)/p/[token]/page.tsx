import { PublicPass } from "@visitors/components/PublicPass";

export default async function VisitorPassPage({
	params,
}: {
	params: Promise<{ token: string }>;
}) {
	const { token } = await params;
	return <PublicPass token={token} />;
}
