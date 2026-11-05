"use client";

import { useSession } from "@auth/hooks/use-session";
import { config } from "@config";
import { useActiveOrganization } from "@organizations/hooks/use-active-organization";
import { config as authConfig } from "@repo/auth/config";
import { Button, cn } from "@repo/ui";
import {
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from "@repo/ui/components/tooltip";
import { UserMenu } from "@shared/components/UserMenu";
import {
	BotMessageSquareIcon,
	ChevronRightIcon,
	HomeIcon,
	PanelLeftCloseIcon,
	PanelLeftOpenIcon,
	SettingsIcon,
	ShieldCheck,
	UserCog2Icon,
	UserCogIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { OrganzationSelect } from "../../organizations/components/OrganizationSelect";
import { useIsMobile } from "../hooks/use-media-query";
import { useSidebar } from "../lib/sidebar-context";

export function NavBar() {
	const t = useTranslations();
	const pathname = usePathname();
	const { user } = useSession();
	const { activeOrganization, isOrganizationAdmin } = useActiveOrganization();
	const { isCollapsed, toggleCollapsed } = useSidebar();
	const isMobile = useIsMobile();

	// Never use collapsed style on mobile - always show expanded
	const isCollapsedEffective = isCollapsed && !isMobile;

	const { useSidebarLayout } = config;

	const basePath = activeOrganization ? `/${activeOrganization.slug}` : "/";

	const menuItems = [
		{
			label: t("app.menu.start"),
			href: basePath,
			icon: HomeIcon,
			isActive: pathname === "/" || pathname === basePath,
		},
		{
			label: t("app.menu.aiChatbot"),
			href: "/chatbot",
			icon: BotMessageSquareIcon,
			isActive: pathname.startsWith("/chatbot"),
		},
		...(authConfig.organizations.enable &&
		activeOrganization &&
		isOrganizationAdmin
			? [
					{
						label: t("app.menu.organizationSettings"),
						href: `${basePath}/settings`,
						icon: SettingsIcon,
						isActive: pathname.startsWith(`${basePath}/settings/`),
					},
				]
			: []),
		{
			label: t("app.menu.accountSettings"),
			href: "/settings",
			icon: UserCog2Icon,
			isActive: pathname.startsWith("/settings/"),
		},
		...(user?.role === "admin"
			? [
					{
						label: t("app.menu.admin"),
						href: "/admin",
						icon: UserCogIcon,
						isActive: pathname.startsWith("/admin/"),
					},
				]
			: []),
	];

	return (
		<nav
			className={cn("w-full", {
				"w-full md:fixed md:top-0 md:left-0 md:h-full md:w-[280px]":
					useSidebarLayout,
				"md:w-[80px]": useSidebarLayout && isCollapsedEffective,
			})}
		>
			<div
				className={cn("container max-w-6xl py-4", {
					"py-4 md:flex md:h-full md:flex-col md:px-4 md:pb-0":
						useSidebarLayout,
				})}
			>
				<div className="flex flex-wrap items-center justify-between gap-6">
					<div
						className={cn("flex items-center gap-6 md:gap-2", {
							"md:flex md:w-full md:flex-col md:items-stretch md:align-stretch":
								useSidebarLayout,
						})}
					>
						<div className="flex items-center gap-2 md:w-full">
							<Link
								href="/"
								className="flex items-center gap-2 rounded-md text-foreground"
								aria-label="Smart Visitors dashboard"
							>
								<ShieldCheck className="size-6 shrink-0 text-primary" />
								{!isCollapsedEffective && (
									<span className="font-semibold tracking-tight">
										Smart Visitors
									</span>
								)}
							</Link>
						</div>

						{authConfig.organizations.enable &&
							!authConfig.organizations.hideOrganization && (
								<>
									{!isCollapsedEffective && (
										<span
											className={cn(
												"hidden opacity-30 md:block",
												{
													"md:hidden":
														useSidebarLayout,
												},
											)}
										>
											<ChevronRightIcon className="size-4" />
										</span>
									)}

									<OrganzationSelect
										className={cn({
											"md:mt-2": useSidebarLayout,
											"md:flex md:justify-center":
												useSidebarLayout &&
												isCollapsedEffective,
										})}
										collapsed={
											isCollapsedEffective &&
											useSidebarLayout
										}
									/>
								</>
							)}
					</div>

					<div
						className={cn(
							"mr-0 ml-auto flex items-center justify-end gap-4",
							{
								"md:hidden": useSidebarLayout,
							},
						)}
					>
						<UserMenu />
					</div>
				</div>

				<TooltipProvider delayDuration={0}>
					<ul
						className={cn(
							"no-scrollbar mt-4 flex flex-nowrap list-none items-center justify-start gap-2 overflow-x-auto text-sm -mx-6 md:mx-0 px-6 md:px-0 md:overflow-visible md:flex-wrap",
							{
								"md:mx-0 md:my-6 md:flex md:flex-col md:flex-nowrap md:items-stretch md:gap-1 md:px-0":
									useSidebarLayout,
								"md:items-center":
									useSidebarLayout && isCollapsedEffective,
							},
						)}
					>
						{menuItems.map((menuItem) => {
							const menuItemContent = (
								<Link
									href={menuItem.href}
									className={cn(
										"flex items-center border border-transparent gap-3 whitespace-nowrap rounded-lg px-3 py-2 transition-colors",
										{
											"font-semibold bg-card border-border":
												menuItem.isActive,
											"hover:bg-accent/50":
												!menuItem.isActive,
											"md:w-full": useSidebarLayout,
											"md:justify-center md:px-2":
												useSidebarLayout &&
												isCollapsedEffective,
										},
									)}
									prefetch
								>
									<menuItem.icon
										className={cn(
											"size-4 shrink-0",
											menuItem.isActive
												? "text-foreground"
												: "text-muted-foreground opacity-60",
										)}
									/>
									{(!isCollapsedEffective ||
										!useSidebarLayout) && (
										<span
											className={cn({
												"text-foreground":
													menuItem.isActive,
												"text-muted-foreground":
													!menuItem.isActive,
											})}
										>
											{menuItem.label}
										</span>
									)}
								</Link>
							);

							if (isCollapsedEffective && useSidebarLayout) {
								return (
									<li key={menuItem.href}>
										<Tooltip>
											<TooltipTrigger asChild>
												{menuItemContent}
											</TooltipTrigger>
											<TooltipContent side="right">
												{menuItem.label}
											</TooltipContent>
										</Tooltip>
									</li>
								);
							}

							return (
								<li key={menuItem.href}>{menuItemContent}</li>
							);
						})}
					</ul>
				</TooltipProvider>

				<div
					className={cn("mt-auto mb-0 hidden flex-col gap-2 pb-4", {
						"md:flex": useSidebarLayout,
						"md:items-center":
							useSidebarLayout && isCollapsedEffective,
					})}
				>
					{useSidebarLayout && (
						<div className="flex justify-end">
							<Button
								variant="ghost"
								size="icon"
								onClick={toggleCollapsed}
								aria-label={
									isCollapsed
										? "Expand sidebar"
										: "Collapse sidebar"
								}
							>
								{isCollapsed ? (
									<PanelLeftOpenIcon className="size-4" />
								) : (
									<PanelLeftCloseIcon className="size-4" />
								)}
							</Button>
						</div>
					)}
					<div
						className={cn({
							"md:flex md:justify-center":
								useSidebarLayout && isCollapsedEffective,
						})}
					>
						<UserMenu showUserName={!isCollapsedEffective} />
					</div>
				</div>
			</div>
		</nav>
	);
}
