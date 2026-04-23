# SPDX-FileCopyrightText: © 2026 DSLab - Fondazione Bruno Kessler
#
# SPDX-License-Identifier: Apache-2.0

import json
from http.server import BaseHTTPRequestHandler, HTTPServer

import numpy as np
import whisper


MODEL_NAME = "tiny"
HOST = "0.0.0.0"
PORT = 8081

# Load once at startup so each request can reuse the same model instance.
MODEL = whisper.load_model(MODEL_NAME)


class WhisperHandler(BaseHTTPRequestHandler):
    def _send_json(self, status: int, payload: dict) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        if self.path == "/health":
            self._send_json(200, {"status": "ok", "model": MODEL_NAME})
            return
        self._send_json(404, {"error": "not found"})

    def do_POST(self) -> None:
        if self.path != "/transcribe":
            self._send_json(404, {"error": "not found"})
            return

        length_header = self.headers.get("Content-Length")
        if not length_header:
            self._send_json(400, {"error": "missing Content-Length"})
            return

        try:
            content_length = int(length_header)
        except ValueError:
            self._send_json(400, {"error": "invalid Content-Length"})
            return

        pcm_bytes = self.rfile.read(content_length)
        if not pcm_bytes:
            self._send_json(400, {"error": "empty request body"})
            return

        try:
            audio = np.frombuffer(pcm_bytes, dtype=np.int16).astype(np.float32) / 32768.0
            result = MODEL.transcribe(audio)
            text = result.get("text", "")
        except Exception as exc:
            self._send_json(500, {"error": f"transcription failed: {exc}"})
            return

        payload = {
            "transcription": text,
            "size": len(pcm_bytes),
            "test": len(text),
        }
        self._send_json(200, payload)


def main() -> None:
    server = HTTPServer((HOST, PORT), WhisperHandler)
    print(f"Whisper test server listening on http://{HOST}:{PORT} (model={MODEL_NAME})")
    server.serve_forever()


if __name__ == "__main__":
    main()
