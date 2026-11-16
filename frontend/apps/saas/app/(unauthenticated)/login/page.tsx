import { GoLoginForm } from "@auth/components/GoLoginForm";
import { getTranslations } from "next-intl/server";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export async function generateMetadata() {
	const t = await getTranslations("auth.login");

	return {
		title: t("title"),
	};
}

export default function LoginPage() {
	return <GoLoginForm />;
}
