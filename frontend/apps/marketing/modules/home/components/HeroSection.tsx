"use client";

import { config } from "@config";
import { Button } from "@repo/ui/components/button";
import { ArrowRightIcon } from "lucide-react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import heroImage from "../../../public/images/hero-image.png";
import heroImageDark from "../../../public/images/hero-image-dark.png";

export function HeroSection() {
	const t = useTranslations();

	return (
		<div className="relative max-w-full overflow-x-hidden bg-linear-to-t from-background via-primary/5 to-background">
			<div className="container relative z-20 py-8 md:py-16 text-center">
				<div className="mb-4 flex justify-center">
					<div className="flex flex-wrap items-center justify-center rounded-full bg-muted p-px px-3 py-1 font-normal text-foreground text-sm">
						<span className="flex items-center gap-2 rounded-full font-semibold">
							{t("home.hero.new")}
						</span>
						<span className="ml-1 block font-medium">
							{t("home.hero.featureBadge")}
						</span>
					</div>
				</div>

				<h1 className="text-balance font-medium text-4xl md:text-5xl lg:text-6xl xl:text-7xl leading-tighter text-foreground mx-auto max-w-3xl">
					{t("home.hero.title")}
				</h1>

				<p className="mt-2 text-foreground/60 text-sm sm:text-lg max-w-3xl mx-auto text-balance">
					{t("home.hero.subtitle")}
				</p>

				<div className="mt-4 flex items-center justify-center gap-2">
					<Button size="lg" variant="primary" asChild>
						<a href={config.saasUrl}>
							{t("home.hero.getStarted")}
							<ArrowRightIcon className="ml-2 size-4" />
						</a>
					</Button>
					{config.docsUrl && (
						<Button variant="ghost" size="lg" asChild>
							<a href={config.docsUrl}>
								{t("home.hero.documentation")}
							</a>
						</Button>
					)}
				</div>

				<div className="mx-auto mt-12 lg:mt-16 lg:flex-1 rounded-4xl bg-primary/5 p-4 border border-primary/10">
					<Image
						src={heroImage}
						alt={t("home.hero.imageAlt")}
						className="block rounded-xl dark:hidden"
						priority
					/>
					<Image
						src={heroImageDark}
						alt={t("home.hero.imageAlt")}
						className="hidden rounded-xl dark:block"
						priority
					/>
				</div>
			</div>
		</div>
	);
}
