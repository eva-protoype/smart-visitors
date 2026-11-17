"use client";

import { Button } from "@repo/ui/components/button";
import { Camera, CameraOff } from "lucide-react";
import { useEffect, useRef, useState } from "react";

interface BarcodeResult {
	rawValue: string;
}

interface BarcodeDetectorLike {
	detect(source: HTMLVideoElement): Promise<BarcodeResult[]>;
}

interface BarcodeDetectorConstructor {
	new (options?: { formats?: string[] }): BarcodeDetectorLike;
}

declare global {
	interface Window {
		BarcodeDetector?: BarcodeDetectorConstructor;
	}
}

export function CameraQrScanner({
	onDetected,
}: {
	onDetected: (value: string) => void;
}) {
	const videoRef = useRef<HTMLVideoElement>(null);
	const streamRef = useRef<MediaStream | null>(null);
	const scanningRef = useRef(false);
	const [cameraOn, setCameraOn] = useState(false);
	const [available, setAvailable] = useState(true);
	const [error, setError] = useState("");

	useEffect(() => stopCamera, []);

	function stopCamera() {
		scanningRef.current = false;
		streamRef.current?.getTracks().forEach((track) => track.stop());
		streamRef.current = null;
		setCameraOn(false);
	}

	async function startCamera() {
		setError("");
		if (!window.BarcodeDetector || !navigator.mediaDevices?.getUserMedia) {
			setAvailable(false);
			setError("Camera QR scanning is not supported in this browser.");
			return;
		}
		try {
			const stream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: { ideal: "environment" } },
				audio: false,
			});
			streamRef.current = stream;
			if (!videoRef.current) {
				stopCamera();
				setError("The camera preview could not be opened.");
				return;
			}
			videoRef.current.srcObject = stream;
			await videoRef.current.play();
			setCameraOn(true);
			scanningRef.current = true;
			const detector = new window.BarcodeDetector({ formats: ["qr_code"] });
			const detect = async () => {
				if (!scanningRef.current || !videoRef.current) return;
				try {
					const codes = await detector.detect(videoRef.current);
					if (codes[0]?.rawValue) {
						const value = codes[0].rawValue;
						stopCamera();
						onDetected(value);
						return;
					}
				} catch {
					stopCamera();
					setError("Could not read the QR code. Use manual token entry.");
					return;
				}
				window.setTimeout(() => void detect(), 250);
			};
			void detect();
		} catch {
			setAvailable(false);
			setError("Camera access was denied or unavailable. Use manual token entry.");
		}
	}

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
					<Button type="button" variant="secondary" onClick={stopCamera}>
						<CameraOff className="size-4" aria-hidden="true" /> Stop camera
					</Button>
				) : (
					<Button type="button" variant="secondary" onClick={() => void startCamera()} disabled={!available}>
						<Camera className="size-4" aria-hidden="true" /> Open camera
					</Button>
				)}
			</div>
			{cameraOn && (
				<div className="mt-4 overflow-hidden rounded-xl bg-slate-950">
					<video ref={videoRef} className="aspect-video w-full object-cover" muted playsInline />
				</div>
			)}
			{error && <p className="mt-3 text-sm text-destructive" role="alert">{error}</p>}
		</div>
	);
}
