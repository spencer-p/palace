console.log("hello from palace.js");

function uploadContent() {
	console.log("Uploading content")
	fetch("https://icebox.spencerjp.dev/palace/pages", {
		method: "POST",
		mode: "no-cors",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			"url": window.location.toString(),
			"title": document.querySelector("title").innerText,
			"text": document.querySelector("body").innerText,
		}),
	})
		.then((response) => console.log(response))
		.catch((err) => console.error("error:", err));
}

if (document.readyState === "loading") {
  // Loading hasn't finished yet.
  document.addEventListener("DOMContentLoaded", uploadContent);
} else {
  // `DOMContentLoaded` has already fired.
  uploadContent();
}
