class AffiliateTracker {
    constructor(options = {}) {
        this.endpoint = options.endpoint || 'https://your-tracker-domain.com';
        this.campaignId = options.campaignId;
        this.clickId = this.getClickId();
    }

    getClickId() {
        const urlParams = new URLSearchParams(window.location.search);
        return urlParams.get('click_id') || '';
    }

    async trackVisit() {
        try {
            const response = await fetch(`${this.endpoint}/track`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    click_id: this.clickId,
                    campaign_id: this.campaignId
                })
            });

            const data = await response.json();
            localStorage.setItem('visitor_id', data.visitor_id);
            
        } catch (error) {
            console.error('Error tracking visit:', error);
        }
    }

    async trackConversion(amount) {
        const visitorId = localStorage.getItem('visitor_id');
        
        try {
            await fetch(`${this.endpoint}/postback`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    visitor_id: visitorId,
                    click_id: this.clickId,
                    campaign_id: this.campaignId,
                    amount: amount
                })
            });
        } catch (error) {
            console.error('Error tracking conversion:', error);
        }
    }
}

window.AffiliateTracker = AffiliateTracker; 