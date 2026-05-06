// extension/background.js

/**
 * Stores the token retrieved from the icebox/palace cookie.
 * Runs periodically or on extension startup.
 */
async function storeToken() {
    console.log("Background: Attempting to retrieve and store palace token.");
    try {
        const cookie = await chrome.cookies.get(
            {
                url: "https://icebox.spencerjp.dev/palace/",
                name: "palace_auth"
            }
        );

        if (cookie) {
            let palace = { token: cookie.value };
            await chrome.storage.local.set({ palace });
            console.log("Background: Successfully stored/updated palace token.");
        } else {
            console.log("Background: Palace cookie not found. Token is not available.");
        }
    } catch (error) {
        console.error("Background: Error while retrieving cookie:", error);
    }
}

// --- Service Worker Initialization ---

/**
 * Sets up the recurrent alarm for token refresh.
 */
function setupAlarms() {
    // Schedule token refresh every day
    chrome.alarms.create("tokenRefresh", { periodInMinutes: 24*60 });

    chrome.alarms.onAlarm.addListener((alarm) => {
        if (alarm.name === "tokenRefresh") {
            console.log("Background: Executing scheduled token refresh.");
            storeToken();
        }
    });
}

// Event Listeners
chrome.runtime.onInstalled.addListener((details) => {
    console.log("Background: Extension installed or updated. Running initial token check.");
    storeToken();

    if (details.reason === 'install' || details.reason === 'update') {
        setupAlarms();
    }
});

chrome.runtime.onStartup.addListener(() => {
    console.log("Background: Extension started up. Running initial token check.");
    storeToken();
    // Ensure alarms are set up if the extension was installed previously but browser was closed
    setupAlarms();
});
