const banlist = [
	"https://icebox.spencerjp.dev/palace.*",
	"https://(www.)?google.com.*",
	"https://(www.)?youtube.com.*",
	"https://xkcd.com.*",
	"(http://)?localhost:.*"
];

const regexBanlist = new RegExp("("+banlist.join("|")+")");
const referrerBanlist = new RegExp("https://icebox.spencerjp.dev/");

// findMainContentOr tries to find a selector for the main content of the page
// by searching for a "Skip to content" link that links to an element with
// non-empty content. If not found, the defaultSelector is returned.
function findMainContentOr(defaultSelector) {
	let walker = document.createTreeWalker(document.body, NodeFilter.SHOW_ELEMENT);

	const skipToMainContent = /^(skip|jump) to (main )?content$/i;
	const containsSkip = /(skip|jump) to (main )?content/i; // Note no ^, $.
	const anchor = /^#.*/;

	let node = walker.nextNode();
	while (node) {
		let href = node.getAttribute("href");
		if (skipToMainContent.test(node.innerText) &&
			anchor.test(href) &&
			document.querySelector(href) &&
			document.querySelector(href).innerText != "") {
			return href;
		}

		// If the skip button is in the current inner text, descend naturally.
		// If not, move to the next sibling.
		if (containsSkip.test(node.innerText)) {
			node = walker.nextNode();
		} else {
			node = walker.nextSibling();
		}
	}
	return defaultSelector;
}

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

	let selector = findMainContentOr("body");
	console.log("palace: scraping text of", selector)

	fetch("https://icebox.spencerjp.dev/palace/pages", {
		method: "POST",
		mode: "no-cors",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			"url": url,
			"title": document.title,
			"text": document.querySelector(selector).innerText,
			"token": opts.palace.token,
		}),
	})
		.then((response) => response.text())
		.then((_) => console.log("palace response: ok"))
		.catch((err) => console.error("palace error:", err));
}

if (!(document.readyState === "loading")) {
  // `DOMContentLoaded` has already fired.
  uploadContent();
}

// Set a listener anyways for fancy pages.
document.addEventListener("DOMContentLoaded", uploadContent);
