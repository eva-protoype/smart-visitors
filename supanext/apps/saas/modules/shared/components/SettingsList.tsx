import type { PropsWithChildren } from "react";
import { Children } from "react";

export function SettingsList({ children }: PropsWithChildren) {
	const validChildren = Children.toArray(children).filter(Boolean);

	if (validChildren.length === 0) {
		return null;
	}

	return (
		<div className="@container flex flex-col gap-3">
			{validChildren.map((child, i) => (
				<div key={`settings-item-${i}`}>{child}</div>
			))}
		</div>
	);
}
