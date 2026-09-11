-- Corporate Billing System
CREATE TABLE IF NOT EXISTS corporate_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES business_accounts(id) ON DELETE CASCADE,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    ride_count INTEGER NOT NULL DEFAULT 0,
    total_cents BIGINT NOT NULL DEFAULT 0,
    tax_cents BIGINT NOT NULL DEFAULT 0,
    grand_total BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sent', 'paid', 'overdue', 'cancelled')),
    due_date TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_corporate_invoices_account ON corporate_invoices(account_id, period_end DESC);
CREATE INDEX idx_corporate_invoices_status ON corporate_invoices(status);

-- Invoice Items
CREATE TABLE IF NOT EXISTS corporate_invoice_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID NOT NULL REFERENCES corporate_invoices(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    ride_id UUID REFERENCES rides(id) ON DELETE SET NULL,
    date TIMESTAMPTZ NOT NULL,
    amount_cents BIGINT NOT NULL,
    cost_center_id UUID REFERENCES cost_centers(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoice_items_invoice ON corporate_invoice_items(invoice_id);

-- Cost Centers
CREATE TABLE IF NOT EXISTS cost_centers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES business_accounts(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(20) NOT NULL,
    budget_cents BIGINT NOT NULL DEFAULT 0,
    spent_cents BIGINT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(account_id, code)
);

CREATE INDEX idx_cost_centers_account ON cost_centers(account_id);

-- Trigger to update timestamps
CREATE OR REPLACE FUNCTION update_billing_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_billing_timestamp
    BEFORE UPDATE ON corporate_invoices
    FOR EACH ROW
    EXECUTE FUNCTION update_billing_timestamp();

CREATE TRIGGER trigger_update_cost_center_timestamp
    BEFORE UPDATE ON cost_centers
    FOR EACH ROW
    EXECUTE FUNCTION update_billing_timestamp();
