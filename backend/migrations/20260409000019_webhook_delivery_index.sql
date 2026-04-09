CREATE INDEX idx_webhook_deliveries_sub ON webhook_deliveries (subscription_id, created_at DESC);
