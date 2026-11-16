"use client";

import { useEffect, useState } from "react";

/**
 * Hook to detect if the viewport matches a media query.
 * Uses Tailwind's md breakpoint (768px) by default for mobile detection.
 */
export function useMediaQuery(query: string): boolean {
	const [matches, setMatches] = useState(false);

	useEffect(() => {
		const mediaQuery = window.matchMedia(query);
		setMatches(mediaQuery.matches);

		const handler = (event: MediaQueryListEvent) => {
			setMatches(event.matches);
		};
		mediaQuery.addEventListener("change", handler);
		return () => mediaQuery.removeEventListener("change", handler);
	}, [query]);

	return matches;
}

/** Returns true when viewport is below md breakpoint (768px) - i.e. mobile */
export function useIsMobile(): boolean {
	return useMediaQuery("(max-width: 767px)");
}
