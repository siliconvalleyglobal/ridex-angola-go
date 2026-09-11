-- name: CreateSAFTInvoice :one
INSERT INTO saft_invoices (ride_id, charge_id, invoice_number, issue_date, due_date, customer_name, customer_tax_id, items, total_cents, currency, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'draft')
RETURNING *;

-- name: GetSAFTInvoiceByID :one
SELECT * FROM saft_invoices
WHERE id = $1;

-- name: GetSAFTInvoiceByNumber :one
SELECT * FROM saft_invoices
WHERE invoice_number = $1;

-- name: IssueSAFTInvoice :one
UPDATE saft_invoices
SET status = 'issued', issued_at = NOW()
WHERE id = $1
RETURNING *;

-- name: MarkSAFTInvoicePaid :one
UPDATE saft_invoices
SET status = 'paid', paid_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetSAFTInvoicesByRide :many
SELECT * FROM saft_invoices
WHERE ride_id = $1
ORDER BY created_at ASC;

-- name: GetSAFTInvoicesByRider :many
SELECT i.*
FROM saft_invoices i
JOIN rides r ON r.id = i.ride_id
WHERE r.rider_id = $1
ORDER BY i.created_at DESC
LIMIT $2;

-- name: GetPendingSAFTInvoices :many
SELECT * FROM saft_invoices
WHERE status = 'issued'
ORDER BY issued_at ASC
LIMIT $1;

-- name: GetSAFTInvoicesByStatus :many
SELECT * FROM saft_invoices
WHERE status = $1
ORDER BY created_at ASC
LIMIT $2;
