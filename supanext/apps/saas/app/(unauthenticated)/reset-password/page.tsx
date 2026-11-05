import { ResetPasswordForm } from "@auth/components/ResetPasswordForm";
import { getTranslations } from "next-intl/server";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export async function generateMetadata() {
	const t = await getTranslations("auth.resetPassword");

	return {
		title: t("title"),
	};
}

export default function ResetPasswordPage() {
	return <ResetPasswordForm />;
}
