-- name: CreateCorporateInvoice :one
INSERT INTO corporate_invoices (account_id, period_start, period_end, ride_count, total_cents, tax_cents, grand_total, status, due_date)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetCorporateInvoiceByID :one
SELECT * FROM corporate_invoices
WHERE id = $1;

-- name: ListCorporateInvoicesByAccount :many
SELECT * FROM corporate_invoices
WHERE account_id = $1
ORDER BY period_end DESC
LIMIT $2 OFFSET $3;

-- name: UpdateCorporateInvoiceStatus :one
UPDATE corporate_invoices
SET status = $2, sent_at = $3, paid_at = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListOverdueInvoices :many
SELECT * FROM corporate_invoices
WHERE status = 'sent' AND due_date < NOW()
ORDER BY due_date ASC;

-- name: AddInvoiceItem :one
INSERT INTO corporate_invoice_items (invoice_id, description, ride_id, date, amount_cents, cost_center_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListInvoiceItems :many
SELECT * FROM corporate_invoice_items
WHERE invoice_id = $1
ORDER BY date DESC;

-- name: CreateCostCenter :one
INSERT INTO cost_centers (account_id, name, code, budget_cents)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetCostCenterByID :one
SELECT * FROM cost_centers
WHERE id = $1;

-- name: ListCostCentersByAccount :many
SELECT * FROM cost_centers
WHERE account_id = $1
ORDER BY name;

-- name: UpdateCostCenterSpending :one
UPDATE cost_centers
SET spent_cents = spent_cents + $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateCostCenter :one
UPDATE cost_centers
SET name = $2, budget_cents = $3, active = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteCostCenter :exec
DELETE FROM cost_centers
WHERE id = $1 AND account_id = $2;
