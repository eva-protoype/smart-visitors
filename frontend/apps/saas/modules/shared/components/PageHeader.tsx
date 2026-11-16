"use client";

import { cn } from "@repo/ui";

export function PageHeader({
	title,
	subtitle,
	className,
}: {
	title: string;
	subtitle?: string;
	className?: string;
}) {
	return (
		<div className={cn("mb-8", className)}>
			<h2 className="font-medium text-2xl lg:text-3xl">{title}</h2>
			<p className="opacity-60">{subtitle}</p>
		</div>
	);
}
