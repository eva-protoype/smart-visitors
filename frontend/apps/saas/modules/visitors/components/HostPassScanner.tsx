"use client";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { Camera, CameraOff, SearchCheck } from "lucide-react";
import { useState } from "react";
import { useQrCamera } from "../lib/use-qr-camera";

interface ScanResult {
	pass_id: string;
	status: string;
	visitor: { name: string };
	host_name: string;
	purpose: string;
	valid_from: string;
	valid_until: string;
}

interface ApiError {
	error?: { message?: string };
}

export function HostPassScanner() {
	const [token, setToken] = useState("");
	const [result, setResult] = useState<ScanResult | null>(null);
	const [error, setError] = useState("");
	const [scanning, setScanning] = useState(false);

	const {
		videoRef,
		cameraOn,
		starting,
		error: cameraError,
		startCamera,
		stopCamera,
	} = useQrCamera({
		onDetected: (value) => {
			void lookup(value);
		},
	});

	async function lookup(rawToken: string) {
		const cleanToken = rawToken.trim();
		if (!cleanToken) {
			setError("Enter or scan a visitor QR token.");
			return;
		}
		setError("");
		setResult(null);
		setScanning(true);
		try {
			const response = await fetch("/api/host/scan", {
				method: "POST",
				credentials: "same-origin",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ token: cleanToken }),
			});
			const body = (await response.json()) as ScanResult & ApiError;
			if (!response.ok) {
				setError(
					body.error?.message ?? "This pass could not be verified.",
				);
				return;
			}
			setToken(cleanToken);
			setResult(body);
			stopCamera();
		} catch {
			setError("The visitor service could not be reached.");
		} finally {
			setScanning(false);
		}
	}

	return (
		<section
			id="host-scan"
			className="rounded-2xl border border-border bg-card p-5 shadow-sm sm:p-6"
		>
			<div className="flex items-start justify-between gap-4">
				<div>
					<h2 className="font-semibold">Verify a visitor pass</h2>
					<p className="mt-1 text-sm text-muted-foreground">
						This scanner accepts only passes assigned to your host
						account.
					</p>
				</div>
				<SearchCheck
					className="size-5 text-[#587662]"
					aria-hidden="true"
				/>
			</div>
			<div
				className={
					cameraOn
						? "mt-5 overflow-hidden rounded-2xl bg-slate-950"
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
			<div className="mt-5 flex flex-wrap gap-3">
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
						<Camera className="size-4" aria-hidden="true" /> Scan
						with camera
					</Button>
				)}
			</div>
			<form
				className="mt-4 flex flex-col gap-3 sm:flex-row"
				onSubmit={(event) => {
					event.preventDefault();
					void lookup(token);
				}}
			>
				<Input
					value={token}
					onChange={(event) => setToken(event.target.value)}
					placeholder="Paste sv1:… token"
					autoComplete="off"
					aria-label="Visitor QR token"
				/>
				<Button type="submit" variant="primary" loading={scanning}>
					Verify pass
				</Button>
			</form>
			{error && (
				<p className="mt-4 text-sm text-destructive" role="alert">
					{error}
				</p>
			)}
			{cameraError && (
				<p className="mt-4 text-sm text-destructive" role="alert">
					{cameraError}
				</p>
			)}
			{result && (
				<div className="mt-5 rounded-xl border border-[#d7e3d8] bg-[#f0f4ef] p-4">
					<div className="flex items-center justify-between gap-3">
						<p className="font-semibold">{result.visitor.name}</p>
						<span className="rounded-full bg-white px-2.5 py-1 text-xs font-medium capitalize">
							{result.status.replace("_", " ")}
						</span>
					</div>
					<p className="mt-2 text-sm text-muted-foreground">
						{result.purpose}
					</p>
					<p className="mt-1 text-xs text-muted-foreground">
						Valid until{" "}
						{new Date(result.valid_until).toLocaleString()}
					</p>
				</div>
			)}
		</section>
	);
}
