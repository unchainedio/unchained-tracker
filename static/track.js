class AffiliateTracker {
    constructor(config = {}) {
        this.endpoint = window.location.origin;
        this.campaignId = config.campaign_id || '';
    }

    async trackVisit() {
        try {
            const response = await fetch(`${this.endpoint}/track`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    click_id: this.clickId,
                    campaign_id: this.campaignId,
                    ...this.deviceInfo,
                    screen_resolution: this.screenInfo.resolution,
                    viewport_size: this.screenInfo.viewport
                })
            });
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const data = await response.json();
            localStorage.setItem("visitor_id", data.visitor_id);
            return data;
        } catch (err) {
            console.error("Error tracking visit:", err);
            throw err;
        }
    }
} 