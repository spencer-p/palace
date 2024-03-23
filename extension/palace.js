
async function uploadContent() {
	const opts = await chrome.storage.local.get("palace");
	if (window.location.toString().startsWith("https://icebox.spencerjp.dev/")) {
		console.log("palace: short-circuit, will not scrape self");
		return;
	}
	fetch("https://icebox.spencerjp.dev/palace/pages", {
		method: "POST",
		mode: "no-cors",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			"url": window.location.toString(),
			"title": document.title,
			"text": document.querySelector("body").innerText,
			"token": opts.palace.token,
		}),
	})
		.then((response) => response.text())
		.then((data) => console.log("palace response:", data))
		.catch((err) => console.error("palace error:", err));
}

if (!(document.readyState === "loading")) {
  // `DOMContentLoaded` has already fired.
  uploadContent();
}

// Set a listener anyways for fancy pages.
document.addEventListener("DOMContentLoaded", uploadContent);
