import { FaqSection } from "@home/components/FaqSection";
import { FeaturesSection } from "@home/components/FeaturesSection";
import { HeroSection } from "@home/components/HeroSection";
import { NewsletterSection } from "@home/components/NewsletterSection";
import { PricingSection } from "@home/components/PricingSection";
import { setRequestLocale } from "next-intl/server";

export default async function Home({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	const { locale } = await params;
	setRequestLocale(locale);

	return (
		<>
			<HeroSection />
			<FeaturesSection />
			<PricingSection />
			<FaqSection />
			<NewsletterSection />
		</>
	);
}
