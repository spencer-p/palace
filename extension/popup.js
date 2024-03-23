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

document.querySelector("#go").addEventListener("click", (event) => {
	storeToken();
});
