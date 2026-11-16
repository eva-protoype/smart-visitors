"use client";

import { cn } from "@repo/ui";
import {
	CloudIcon,
	ComputerIcon,
	SmartphoneIcon,
	StarIcon,
	WandIcon,
} from "lucide-react";
import Image from "next/image";
import { useTranslations } from "next-intl";
import type { JSXElementConstructor, ReactNode } from "react";

interface FeatureTab {
	id: string;
	title: string;
	icon: JSXElementConstructor<{
		className?: string;
		width?: string;
		height?: string;
	}>;
	subtitle?: string;
	description?: ReactNode;
	image?: string;
	imageBorder?: boolean;
	stack?: {
		title: string;
		href: string;
		icon: JSXElementConstructor<any>;
	}[];
	highlights?: {
		title: string;
		description: string;
		icon: JSXElementConstructor<any>;
		demoLink?: string;
		docsUrl?: string;
	}[];
}

export function FeaturesSection() {
	const t = useTranslations();

	const featureTabs: FeatureTab[] = [
		{
			id: "feature1",
			title: t("home.features.feature1.title"),
			icon: StarIcon,
			subtitle: t("home.features.feature1.subtitle"),
			description: t("home.features.feature1.description"),
			stack: [],
			image: "https://placehold.co/1280x720/EAF2FC/3875C8.png",
			imageBorder: false,
			highlights: [
				{
					title: t("home.features.feature1.benefit1.title"),
					description: t(
						"home.features.feature1.benefit1.description",
					),
					icon: WandIcon,
				},
				{
					title: t("home.features.feature1.benefit2.title"),
					description: t(
						"home.features.feature1.benefit2.description",
					),
					icon: ComputerIcon,
				},
				{
					title: t("home.features.feature1.benefit3.title"),
					description: t(
						"home.features.feature1.benefit3.description",
					),
					icon: SmartphoneIcon,
				},
			],
		},
		{
			id: "feature2",
			title: t("home.features.feature2.title"),
			icon: CloudIcon,
			subtitle: t("home.features.feature2.subtitle"),
			description: t("home.features.feature2.description"),
			stack: [],
			image: "https://placehold.co/1280x720/F0F6FD/2D5FA3.png",
			imageBorder: false,
			highlights: [
				{
					title: t("home.features.feature2.benefit1.title"),
					description: t(
						"home.features.feature2.benefit1.description",
					),
					icon: WandIcon,
				},
				{
					title: t("home.features.feature2.benefit2.title"),
					description: t(
						"home.features.feature2.benefit2.description",
					),
					icon: ComputerIcon,
				},
				{
					title: t("home.features.feature2.benefit3.title"),
					description: t(
						"home.features.feature2.benefit3.description",
					),
					icon: SmartphoneIcon,
				},
			],
		},
	];

	return (
		<section id="features" className="scroll-my-20 py-12 lg:py-16">
			<div className="container">
				<div className="mb-6 lg:mb-0 max-w-3xl mx-auto text-center">
					<small className="font-medium text-xs uppercase tracking-wider text-primary mb-4 block">
						{t("home.features.badge")}
					</small>
					<h2 className="text-3xl lg:text-4xl xl:text-5xl font-medium">
						{t("home.features.title")}
					</h2>
					<p className="mt-2 text-base lg:text-lg text-foreground/60 text-balance">
						{t("home.features.description")}
					</p>
				</div>
			</div>

			<div>
				<div className="container mt-8 lg:mt-12 grid grid-cols-1 gap-8">
					{featureTabs.map((tab, index) => {
						const filteredStack = tab.stack || [];
						const filteredHighlights = tab.highlights || [];
						const isReversed = index % 2 === 1;
						return (
							<div
								key={tab.id}
								className="bg-card border rounded-4xl p-6 md:p-8 lg:p-12"
							>
								<div className="grid grid-cols-1 items-center gap-8 md:grid-cols-2">
									<div
										className={cn("flex justify-center", {
											"md:order-2": isReversed,
										})}
									>
										{tab.image && (
											<div className="w-full max-w-xl">
												<Image
													src={tab.image}
													alt={tab.title}
													width={1280}
													height={720}
													className={cn(
														"aspect-video h-auto w-full rounded-xl object-cover",
														{
															"border-4":
																tab.imageBorder,
														},
													)}
												/>
											</div>
										)}
									</div>

									<div
										className={cn({
											"md:order-1": isReversed,
										})}
									>
										<h3 className="font-normal text-lg text-foreground leading-tight md:text-xl lg:text-2xl">
											<span className="font-medium">
												{tab.title}.{" "}
											</span>
											<span className="font-sans">
												{tab.subtitle}
											</span>
										</h3>

										{tab.description && (
											<p className="mt-4 text-foreground/60">
												{tab.description}
											</p>
										)}

										{filteredStack?.length > 0 && (
											<div className="mt-4 flex flex-wrap gap-6">
												{filteredStack.map(
													(tool, k) => (
														<a
															href={tool.href}
															target="_blank"
															key={`stack-tool-${k}`}
															className="flex items-center gap-2"
															rel="noreferrer"
														>
															<tool.icon className="size-6" />
															<strong className="block text-sm">
																{tool.title}
															</strong>
														</a>
													),
												)}
											</div>
										)}
									</div>
								</div>

								{filteredHighlights.length > 0 && (
									<div className="mt-8 grid gap-6 sm:grid-cols-2 lg:grid-cols-3 lg:mt-12">
										{filteredHighlights.map(
											(highlight, k) => (
												<div
													key={`highlight-${k}`}
													className="flex flex-col items-stretch justify-between rounded-2xl p-4 lg:p-6 bg-background"
												>
													<div>
														<highlight.icon
															className="text-primary text-xl"
															width="1em"
															height="1em"
														/>
														<strong className="mt-2 block font-medium text-lg">
															{highlight.title}
														</strong>
														<p className="mt-1 text-sm">
															{
																highlight.description
															}
														</p>
													</div>
												</div>
											),
										)}
									</div>
								)}
							</div>
						);
					})}
				</div>
			</div>
		</section>
	);
}
