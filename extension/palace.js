const banlist = [
	"https://icebox.spencerjp.dev/palace.*",
	"https://(www.)?google.com.*",
	"https://(www.)?youtube.com.*",
	"https://xkcd.com.*"
];

const regexBanlist = new RegExp("("+banlist.join("|")+")");
const referrerBanlist = new RegExp("https://icebox.spencerjp.dev/");

async function uploadContent() {
	const opts = await chrome.storage.local.get("palace");
	const url = document.URL;
	if (regexBanlist.test(url)) {
		console.log("palace: will not scrape banned url:", url);
		return;
	}

	if (referrerBanlist.test(document.referrer)) {
		console.log("palace: will not scrape because of referrer:", document.referrer);
		return;
	}

	fetch("https://icebox.spencerjp.dev/palace/pages", {
		method: "POST",
		mode: "no-cors",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			"url": url,
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
