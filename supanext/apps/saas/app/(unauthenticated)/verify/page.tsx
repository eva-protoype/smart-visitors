import { OtpForm } from "@auth/components/OtpForm";
import { getTranslations } from "next-intl/server";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export async function generateMetadata() {
	const t = await getTranslations("auth.verify");

	return {
		title: t("title"),
	};
}

export default function VerifyPage() {
	return <OtpForm />;
}
