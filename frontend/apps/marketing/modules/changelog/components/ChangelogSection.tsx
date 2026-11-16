"use client";

import { useFormatter } from "next-intl";
import type { ReactNode } from "react";

export type ChangelogItem = {
	date: string;
	title: string;
	changes: ReactNode[];
};

export function ChangelogSection() {
	const formatter = useFormatter();

	const items: ChangelogItem[] = [
		{
			date: "2026-01-30",
			title: "Performance Improvements",
			changes: ["🚀 Improved performance"],
		},
		{
			date: "2026-01-26",
			title: "Design Updates",
			changes: ["🎨 Updated design", "🐞 Fixed a bug"],
		},
		{
			date: "2026-01-12",
			title: "New Features",
			changes: ["🎉 Added new feature", "🐞 Fixed a bug"],
		},
	];

	return (
		<section id="changelog">
			<div className="mx-auto grid w-full max-w-xl grid-cols-1 gap-4 text-left">
				{items?.map((item, i) => (
					<div key={i} className="rounded-3xl bg-muted p-6 border">
						<div className="flex flex-col items-start gap-1">
							<small className="font-medium text-primary uppercase tracking-wide text-xs whitespace-nowrap">
								{formatter.dateTime(new Date(item.date))}
							</small>

							<h2 className="text-xl font-semibold">
								{item.title}
							</h2>
						</div>
						<ul className="mt-4 list-disc space-y-2 pl-6">
							{item.changes.map((change, j) => (
								<li key={j}>{change}</li>
							))}
						</ul>
					</div>
				))}
			</div>
		</section>
	);
}
