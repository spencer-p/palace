async function storeToken() {
	chrome.cookies.get(
		{
			url: "https://icebox.spencerjp.dev/palace/",
			name: "palace_auth"
		},
		function(cookie) {
			let palace = { token: cookie.value }
			chrome.storage.local.set({palace});
		}
	);
}

// lifted from palace.js
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

// lifted from palace.js
async function uploadContent() {
	const opts = await chrome.storage.local.get("palace");
	const url = document.URL;

	let selector = findMainContentOr("body");
	console.log("palace: scraping text of", selector)

	fetch("https://icebox.spencerjp.dev/palace/pages", {
		method: "POST",
		mode: "cors",
		credentials: "include",
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
		.then((response) => response.json())
		.then((json) => console.log("palace response:", json))
		.catch((err) => console.error("palace error:", err));
}

document.querySelector("#go").addEventListener("click", (event) => {
	storeToken();
});

document.querySelector("#scrape").addEventListener("click", (event) => {
	chrome.tabs.query({active:true, currentWindow:true},tabs => {
		chrome.scripting.executeScript({
			target: { tabId: tabs[0].id },
			function: uploadContent,
		})
	});
});
