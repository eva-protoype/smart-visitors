"use client";

import {
	type RefObject,
	useCallback,
	useEffect,
	useRef,
	useState,
} from "react";

interface UseQrCameraOptions {
	onDetected: (value: string) => void;
}

interface UseQrCameraResult {
	videoRef: RefObject<HTMLVideoElement | null>;
	cameraOn: boolean;
	starting: boolean;
	error: string;
	startCamera: () => Promise<void>;
	stopCamera: () => void;
}

export function useQrCamera({
	onDetected,
}: UseQrCameraOptions): UseQrCameraResult {
	const videoRef = useRef<HTMLVideoElement>(null);
	const streamRef = useRef<MediaStream | null>(null);
	const timerRef = useRef<number | null>(null);
	const scanningRef = useRef(false);
	const [cameraOn, setCameraOn] = useState(false);
	const [starting, setStarting] = useState(false);
	const [error, setError] = useState("");

	const release = useCallback(() => {
		scanningRef.current = false;
		if (timerRef.current !== null) {
			window.clearTimeout(timerRef.current);
			timerRef.current = null;
		}
		for (const track of streamRef.current?.getTracks() ?? []) {
			track.stop();
		}
		streamRef.current = null;
		if (videoRef.current) {
			videoRef.current.srcObject = null;
		}
	}, []);

	const stopCamera = useCallback(() => {
		release();
		setCameraOn(false);
		setStarting(false);
	}, [release]);

	useEffect(() => release, [release]);

	const startCamera = useCallback(async () => {
		if (!navigator.mediaDevices?.getUserMedia) {
			setError(
				"Camera access is not supported in this browser. Use manual entry instead.",
			);
			return;
		}

		setError("");
		setStarting(true);
		try {
			const stream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: { ideal: "environment" } },
				audio: false,
			});
			streamRef.current = stream;

			const video = videoRef.current;
			if (!video) {
				throw new Error("Camera preview is unavailable.");
			}
			video.srcObject = stream;
			await video.play();
			setCameraOn(true);
			setStarting(false);

			const { default: jsQR } = await import("jsqr");
			const canvas = document.createElement("canvas");
			const context = canvas.getContext("2d", {
				willReadFrequently: true,
			});
			if (!context) {
				throw new Error("Could not prepare the camera preview.");
			}
			scanningRef.current = true;

			const detect = () => {
				if (!scanningRef.current) {
					return;
				}

				const currentVideo = videoRef.current;
				if (
					!currentVideo ||
					currentVideo.readyState < HTMLMediaElement.HAVE_CURRENT_DATA
				) {
					timerRef.current = window.setTimeout(detect, 250);
					return;
				}

				const width = currentVideo.videoWidth;
				const height = currentVideo.videoHeight;
				if (!width || !height) {
					timerRef.current = window.setTimeout(detect, 250);
					return;
				}

				const scale = Math.min(1, 720 / Math.max(width, height));
				canvas.width = Math.max(1, Math.round(width * scale));
				canvas.height = Math.max(1, Math.round(height * scale));
				context.drawImage(
					currentVideo,
					0,
					0,
					canvas.width,
					canvas.height,
				);
				const frame = context.getImageData(
					0,
					0,
					canvas.width,
					canvas.height,
				);
				const code = jsQR(frame.data, frame.width, frame.height, {
					inversionAttempts: "dontInvert",
				});

				if (code?.data) {
					stopCamera();
					onDetected(code.data);
					return;
				}

				timerRef.current = window.setTimeout(detect, 200);
			};

			timerRef.current = window.setTimeout(detect, 200);
		} catch (cause) {
			release();
			setCameraOn(false);
			setStarting(false);
			setError(
				cause instanceof DOMException &&
					cause.name === "NotAllowedError"
					? "Camera access was denied. Allow camera permission and try again, or use manual entry."
					: "Camera access was unavailable. Use manual entry instead.",
			);
		}
	}, [onDetected, release, stopCamera]);

	return { videoRef, cameraOn, starting, error, startCamera, stopCamera };
}
