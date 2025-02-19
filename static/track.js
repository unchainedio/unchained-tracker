class AffiliateTracker {
    constructor(config = {}) {
        this.baseUrl = window.location.origin;
        this.campaignId = config.campaign_id || '';
    }
} 