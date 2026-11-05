"use client";

import { cn } from "@repo/ui";
import {
	Accordion,
	AccordionContent,
	AccordionItem,
	AccordionTrigger,
} from "@repo/ui/components/accordion";
import { useTranslations } from "next-intl";

const FAQ_ITEM_KEYS = [
	"refundPolicy",
	"cancelSubscription",
	"changePlan",
	"freeTrial",
] as const;

export function FaqSection({ className }: { className?: string }) {
	const t = useTranslations();

	const items = FAQ_ITEM_KEYS.map((key) => ({
		question: t(`faq.items.${key}.question`),
		answer: t(`faq.items.${key}.answer`),
	}));

	return (
		<section
			className={cn("scroll-mt-20 py-12 lg:py-16", className)}
			id="faq"
		>
			<div className="container">
				<div className="grid grid-cols-1 gap-6 md:gap-8 lg:gap-12 max-w-2xl mx-auto">
					<div className="text-center">
						<h1 className="font-medium text-2xl md:text-3xl lg:text-4xl xl:text-5xl leading-tight text-foreground">
							{t("faq.title")}
						</h1>
						<p className="text-foreground/60 text-sm sm:text-lg mt-2">
							{t("faq.description")}
						</p>
					</div>
					<Accordion
						type="single"
						collapsible
						className="w-full space-y-2 text-left"
					>
						{items.map((item, i) => (
							<AccordionItem
								key={`faq-item-${i}`}
								value={`item-${i}`}
								className="rounded-lg bg-card shadow-none border px-4 lg:px-6"
							>
								<AccordionTrigger className="text-left font-medium text-base hover:no-underline">
									{item.question}
								</AccordionTrigger>
								<AccordionContent className="text-foreground/60">
									{item.answer}
								</AccordionContent>
							</AccordionItem>
						))}
					</Accordion>
				</div>
			</div>
		</section>
	);
}
