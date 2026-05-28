export async function sha256Buffer(buffer) {
  const digest = await crypto.subtle.digest("SHA-256", buffer);
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

export async function sha256String(text) {
  return sha256Buffer(new TextEncoder().encode(text));
}
