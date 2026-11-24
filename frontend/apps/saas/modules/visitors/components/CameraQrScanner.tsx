"use client";

import { Button } from "@repo/ui/components/button";
import { Camera, CameraOff } from "lucide-react";
import { useQrCamera } from "../lib/use-qr-camera";

export function CameraQrScanner({
	onDetected,
}: {
	onDetected: (value: string) => void;
}) {
	const { videoRef, cameraOn, starting, error, startCamera, stopCamera } =
		useQrCamera({ onDetected });

	return (
		<div className="mt-5 rounded-xl border border-[#d7e3d8] bg-[#f7faf6] p-4">
			<div className="flex flex-wrap items-center justify-between gap-3">
				<div>
					<p className="font-medium">Scan QR with camera</p>
					<p className="mt-1 text-xs text-muted-foreground">
						Use HTTPS or localhost and allow camera access.
					</p>
				</div>
				{cameraOn ? (
					<Button
						type="button"
						variant="secondary"
						onClick={stopCamera}
					>
						<CameraOff className="size-4" aria-hidden="true" /> Stop
						camera
					</Button>
				) : (
					<Button
						type="button"
						variant="secondary"
						onClick={() => void startCamera()}
						loading={starting}
					>
						<Camera className="size-4" aria-hidden="true" /> Open
						camera
					</Button>
				)}
			</div>
			<div
				className={
					cameraOn
						? "mt-4 overflow-hidden rounded-xl bg-slate-950"
						: "hidden"
				}
			>
				<video
					ref={videoRef}
					className="aspect-video w-full object-cover"
					muted
					playsInline
					autoPlay
				/>
			</div>
			{error && (
				<p className="mt-3 text-sm text-destructive" role="alert">
					{error}
				</p>
			)}
		</div>
	);
}
